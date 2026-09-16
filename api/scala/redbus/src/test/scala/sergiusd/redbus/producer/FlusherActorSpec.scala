package sergiusd.redbus.producer

import akka.actor.{ActorSystem, Props}
import akka.testkit.TestKit
import org.scalatest.BeforeAndAfterAll
import org.scalatest.concurrent.Eventually
import org.scalatest.matchers.should.Matchers
import org.scalatest.time.{Millis, Seconds, Span}
import org.scalatest.wordspec.AnyWordSpecLike
import sergiusd.redbus.api

import java.util.concurrent.atomic.AtomicInteger
import scala.concurrent.{Future, Promise}

class FlusherActorSpec
  extends TestKit(ActorSystem("FlusherActorSpec"))
  with AnyWordSpecLike
  with Matchers
  with Eventually
  with BeforeAndAfterAll {

  implicit override val patienceConfig: PatienceConfig =
    PatienceConfig(timeout = Span(5, Seconds), interval = Span(50, Millis))

  override def afterAll(): Unit = TestKit.shutdownActorSystem(system)

  private class InMemoryStore(initial: Seq[PublishingMessage]) extends Flusher.Store {
    @volatile var rows: Seq[PublishingMessage] = initial
    @volatile var fetchLimits: Vector[Int] = Vector.empty
    @volatile var deleteCalls: Vector[Seq[Long]] = Vector.empty

    override def fetchBatch(batchSize: Int): Future[Seq[PublishingMessage]] = synchronized {
      fetchLimits = fetchLimits :+ batchSize
      Future.successful(rows.sortBy(_.id).take(batchSize))
    }

    override def deleteBatch(ids: Seq[Long]): Future[Int] = synchronized {
      deleteCalls = deleteCalls :+ ids
      val before = rows.size
      rows = rows.filterNot(message => ids.contains(message.id))
      Future.successful(before - rows.size)
    }
  }

  private def message(
    id: Long,
    topic: String = "topic",
    options: PublishingMessage.Options = PublishingMessage.Options.empty,
  ): PublishingMessage = PublishingMessage(topic, s"payload-$id".getBytes, options, id)

  "FlusherActor" should {

    "fetch bounded rows, preserve id order and split batches at topic boundaries" in {
      val store = new InMemoryStore(Seq(
        message(4, "topic-a"),
        message(2, "topic-a"),
        message(1, "topic-a", PublishingMessage.Options(
          key = Some("message-key"),
          version = Some(7),
          idempotencyKey = Some("idempotency-key"),
          timestamp = Some("2026-09-16T07:00:00Z"),
        )),
        message(3, "topic-b"),
      ))
      @volatile var requests = Vector.empty[api.ProduceBatchRequest]
      val produceBatch: api.ProduceBatchRequest => Future[api.ProduceBatchResponse] = request => synchronized {
        requests = requests :+ request
        Future.successful(api.ProduceBatchResponse(ok = true))
      }
      val actor = system.actorOf(Props(new FlusherActor(store, produceBatch, _ => (), batchSize = 3)))

      actor ! ProcessMessage("sweep")

      eventually(store.rows shouldBe empty)
      requests.map(_.topic) shouldBe Seq("topic-a", "topic-b", "topic-a")
      requests.map(_.messageList.map(_.version)) shouldBe Seq(Seq(7L, 2L), Seq(3L), Seq(4L))
      val first = requests.head.messageList.head
      first.key shouldBe "message-key"
      first.message.toStringUtf8 shouldBe "payload-1"
      first.idempotencyKey shouldBe "idempotency-key"
      first.timestamp shouldBe "2026-09-16T07:00:00Z"
      store.deleteCalls shouldBe Seq(Seq(1L, 2L), Seq(3L), Seq(4L))
      store.fetchLimits.distinct shouldBe Seq(3)
    }

    "delete no rows until the whole batch is confirmed" in {
      val store = new InMemoryStore(Seq(message(1), message(2)))
      val response = Promise[api.ProduceBatchResponse]()
      val attempts = new AtomicInteger(0)
      val produceBatch: api.ProduceBatchRequest => Future[api.ProduceBatchResponse] = _ => {
        attempts.incrementAndGet()
        response.future
      }
      val actor = system.actorOf(Props(new FlusherActor(store, produceBatch, _ => (), batchSize = 100)))

      actor ! ProcessMessage("sweep")
      eventually(attempts.get() shouldBe 1)
      store.rows.map(_.id) shouldBe Seq(1L, 2L)
      store.deleteCalls shouldBe empty

      response.success(api.ProduceBatchResponse(ok = true))
      eventually {
        store.rows shouldBe empty
        store.deleteCalls shouldBe Seq(Seq(1L, 2L))
      }
    }

    "keep the whole batch and retry it on the next trigger after publish failure" in {
      val store = new InMemoryStore(Seq(message(1), message(2)))
      val attempts = new AtomicInteger(0)
      val produceBatch: api.ProduceBatchRequest => Future[api.ProduceBatchResponse] = _ => {
        if (attempts.incrementAndGet() == 1) Future.failed(new RuntimeException("bus unavailable"))
        else Future.successful(api.ProduceBatchResponse(ok = true))
      }
      val logged = new AtomicInteger(0)
      val logger: String => Unit = _ => { logged.incrementAndGet(); () }
      val actor = system.actorOf(Props(new FlusherActor(store, produceBatch, logger, batchSize = 100)))

      actor ! ProcessMessage("notify")
      eventually(attempts.get() shouldBe 1)
      eventually(logged.get() should be >= 1)
      store.rows.map(_.id) shouldBe Seq(1L, 2L)
      store.deleteCalls shouldBe empty

      actor ! ProcessMessage("sweep")
      eventually {
        attempts.get() shouldBe 2
        store.rows shouldBe empty
      }
      store.deleteCalls shouldBe Seq(Seq(1L, 2L))
    }

    "keep the whole batch when the bus rejects the response" in {
      val store = new InMemoryStore(Seq(message(1), message(2)))
      val attempts = new AtomicInteger(0)
      val produceBatch: api.ProduceBatchRequest => Future[api.ProduceBatchResponse] = _ => {
        if (attempts.incrementAndGet() == 1) Future.successful(api.ProduceBatchResponse(ok = false))
        else Future.successful(api.ProduceBatchResponse(ok = true))
      }
      val actor = system.actorOf(Props(new FlusherActor(store, produceBatch, _ => (), batchSize = 100)))

      actor ! ProcessMessage("notify")
      eventually(attempts.get() shouldBe 1)
      store.rows.map(_.id) shouldBe Seq(1L, 2L)
      store.deleteCalls shouldBe empty

      actor ! ProcessMessage("sweep")
      eventually {
        attempts.get() shouldBe 2
        store.rows shouldBe empty
      }
    }

    "reject a non-positive batch size before starting resources" in {
      val produceBatch: api.ProduceBatchRequest => Future[api.ProduceBatchResponse] =
        _ => Future.successful(api.ProduceBatchResponse(ok = true))

      an[IllegalArgumentException] should be thrownBy {
        Flusher.start(null, produceBatch, _ => (), batchSize = 0)(system)
      }
    }

    "remember a trigger received while a failing batch is in progress" in {
      val store = new InMemoryStore(Seq(message(1)))
      val firstResponse = Promise[api.ProduceBatchResponse]()
      val attempts = new AtomicInteger(0)
      val produceBatch: api.ProduceBatchRequest => Future[api.ProduceBatchResponse] = _ => {
        if (attempts.incrementAndGet() == 1) firstResponse.future
        else Future.successful(api.ProduceBatchResponse(ok = true))
      }
      val actor = system.actorOf(Props(new FlusherActor(store, produceBatch, _ => (), batchSize = 100)))

      actor ! ProcessMessage("notify-1")
      eventually(attempts.get() shouldBe 1)
      actor ! ProcessMessage("notify-2")
      firstResponse.failure(new RuntimeException("temporary failure"))

      eventually {
        attempts.get() shouldBe 2
        store.rows shouldBe empty
      }
    }
  }
}
