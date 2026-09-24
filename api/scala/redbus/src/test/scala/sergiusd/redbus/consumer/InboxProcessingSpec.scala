package sergiusd.redbus.consumer

import org.scalatest.matchers.should.Matchers
import org.scalatest.wordspec.AnyWordSpec

import scala.concurrent.duration._
import scala.concurrent.{Await, ExecutionContext, Future}

class InboxProcessingSpec extends AnyWordSpec with Matchers {

  implicit private val ec: ExecutionContext = ExecutionContext.global

  private class InMemoryStore(initial: Set[String] = Set.empty) extends InboxProcessing.Store {
    @volatile var marks: Set[String] = initial
    @volatile var setCalls: Vector[String] = Vector.empty

    override def isProcessed(group: String, topic: String, idempotencyKey: String): Future[Boolean] =
      Future.successful(marks.contains(s"$group|$topic|$idempotencyKey"))

    override def setProcessed(group: String, topic: String, idempotencyKey: String): Future[Unit] = synchronized {
      val key = s"$group|$topic|$idempotencyKey"
      setCalls = setCalls :+ key
      marks = marks + key
      Future.unit
    }
  }

  private class RecordingProcessor(result: Future[Unit] = Future.unit) extends Model.Processor {
    @volatile var calls: Vector[Model.MessageMeta] = Vector.empty
    override def apply(data: Array[Byte], meta: Model.MessageMeta): Future[Unit] = synchronized {
      calls = calls :+ meta
      result
    }
  }

  private def run(
    store: Option[InboxProcessing.Store],
    transactional: Boolean,
    processor: Model.Processor,
    idempotencyKey: String = "key-1",
  ): Either[Throwable, Unit] = Await.result(
    InboxProcessing.process(
      store = store,
      transactional = transactional,
      group = "group",
      topic = "topic",
      idempotencyKey = idempotencyKey,
      data = Array.emptyByteArray,
      meta = Model.MessageMeta(version = Some(3)),
      processor = processor,
      log = _ => (),
    ),
    5.seconds,
  )

  "InboxProcessing in the transactional inbox mode" should {
    "hand the claim to the processor and never write the processed mark" in {
      val store = new InMemoryStore
      val processor = new RecordingProcessor

      run(Some(store), transactional = true, processor) shouldBe Right(())

      processor.calls should have size 1
      processor.calls.head.version shouldBe Some(3)
      processor.calls.head.claimDba shouldBe defined
      store.setCalls shouldBe empty
    }

    "not write the mark after a failure either" in {
      val store = new InMemoryStore
      val processor = new RecordingProcessor(Future.failed(new RuntimeException("boom")))

      run(Some(store), transactional = true, processor).isLeft shouldBe true
      store.setCalls shouldBe empty
    }

    "skip a message that is already marked without calling the processor" in {
      val store = new InMemoryStore(Set("group|topic|key-1"))
      val processor = new RecordingProcessor

      run(Some(store), transactional = true, processor) shouldBe Right(())
      processor.calls shouldBe empty
    }
  }

  "InboxProcessing in the only-once mode" should {
    "pass no claim and write the mark after success" in {
      val store = new InMemoryStore
      val processor = new RecordingProcessor

      run(Some(store), transactional = false, processor) shouldBe Right(())

      processor.calls.map(_.claimDba) shouldBe Vector(None)
      store.setCalls shouldBe Vector("group|topic|key-1")
    }

    "not write the mark after a failure" in {
      val store = new InMemoryStore
      val processor = new RecordingProcessor(Future.failed(new RuntimeException("boom")))

      run(Some(store), transactional = false, processor).isLeft shouldBe true
      store.setCalls shouldBe empty
    }
  }

  "InboxProcessing without an inbox" should {
    "call the processor without a claim" in {
      val processor = new RecordingProcessor

      run(None, transactional = false, processor) shouldBe Right(())
      processor.calls.map(_.claimDba) shouldBe Vector(None)
    }
  }

  "Consumer options" should {
    val init = Model.Listener(consumeTimeout = 1.second, batchSize = 1, unavailableTimeout = 1.second)
    // Never connected: the options only carry the handle.
    val db = slick.jdbc.PostgresProfile.backend.Database.forURL(LocalPostgres.JdbcUrl, driver = "org.postgresql.Driver")

    "enable the transactional inbox only through WithTransactionalInbox, last option winning" in {
      init.transactionalInbox shouldBe false
      Option.WithOnlyOnceProcessor(db)(init).transactionalInbox shouldBe false
      val transactional = Option.WithTransactionalInbox(db)(init)
      transactional.transactionalInbox shouldBe true
      transactional.checkEventProcessedDatabase shouldBe Some(db)
      Option.WithOnlyOnceProcessor(db)(transactional).transactionalInbox shouldBe false
      db.close()
    }
  }
}
