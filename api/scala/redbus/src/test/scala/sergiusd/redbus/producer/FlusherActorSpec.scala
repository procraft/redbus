package sergiusd.redbus.producer

import org.apache.pekko.actor.{ActorSystem, Props}
import org.apache.pekko.testkit.TestKit
import org.scalatest.BeforeAndAfterAll
import org.scalatest.concurrent.Eventually
import org.scalatest.matchers.should.Matchers
import org.scalatest.time.{Millis, Seconds, Span}
import org.scalatest.wordspec.AnyWordSpecLike
import sergiusd.redbus.api

import java.util.concurrent.atomic.AtomicInteger
import scala.concurrent.duration._
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

    @volatile var fetchFailed: Vector[Set[String]] = Vector.empty

    override def fetchBatch(batchSize: Int, failedTopics: Set[String]): Future[Seq[PublishingMessage]] = synchronized {
      fetchLimits = fetchLimits :+ batchSize
      fetchFailed = fetchFailed :+ failedTopics
      Future.successful(rows.filterNot(message => failedTopics.contains(message.topic)).sortBy(_.id).take(batchSize))
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

    "send each topic of the bounded queue head as one batch and preserve id order within the topic" in {
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
      store.fetchFailed.distinct shouldBe Seq(Set.empty[String])
    }

    "group interleaved topics of one fetch into one batch per topic" in {
      val store = new InMemoryStore(Seq(
        message(1, "topic-a"), message(2, "topic-b"), message(3, "topic-a"), message(4, "topic-b"),
      ))
      @volatile var requests = Vector.empty[api.ProduceBatchRequest]
      val produceBatch: api.ProduceBatchRequest => Future[api.ProduceBatchResponse] = request => synchronized {
        requests = requests :+ request
        Future.successful(api.ProduceBatchResponse(ok = true))
      }
      val actor = system.actorOf(Props(new FlusherActor(store, produceBatch, _ => (), batchSize = 100)))

      actor ! ProcessMessage("sweep")

      eventually(store.rows shouldBe empty)
      requests.map(_.topic) shouldBe Seq("topic-a", "topic-b")
      requests.map(_.messageList.map(_.version)) shouldBe Seq(Seq(1L, 3L), Seq(2L, 4L))
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

    "report a failed pass with its cause through the error sink, not the debug logger" in {
      val store = new InMemoryStore(Seq(message(1), message(2)))
      val cause = new RuntimeException("[29] TOPIC_AUTHORIZATION_FAILED")
      val produceBatch: api.ProduceBatchRequest => Future[api.ProduceBatchResponse] =
        _ => Future.failed(cause)
      @volatile var debug = Vector.empty[String]
      @volatile var errors = Vector.empty[(String, Throwable)]
      val logger: String => Unit = line => synchronized { debug = debug :+ line }
      val errorLogger: Flusher.ErrorLogger = (line, e) => synchronized { errors = errors :+ (line -> e) }
      val actor = system.actorOf(Props(new FlusherActor(store, produceBatch, logger, errorLogger, batchSize = 100)))

      actor ! ProcessMessage("notify")
      eventually(errors should have size 1)
      val (line, reported) = errors.head
      line should include("Flush failed (notify) for topic topic")
      reported shouldBe theSameInstanceAs(cause)
      debug.filter(_.contains("Flush failed")) shouldBe empty
      store.rows.map(_.id) shouldBe Seq(1L, 2L)
    }

    "report a rejected batch through the error sink" in {
      val store = new InMemoryStore(Seq(message(1)))
      val produceBatch: api.ProduceBatchRequest => Future[api.ProduceBatchResponse] =
        _ => Future.successful(api.ProduceBatchResponse(ok = false))
      @volatile var errors = Vector.empty[(String, Throwable)]
      val errorLogger: Flusher.ErrorLogger = (line, e) => synchronized { errors = errors :+ (line -> e) }
      val actor = system.actorOf(Props(new FlusherActor(store, produceBatch, _ => (), errorLogger, batchSize = 100)))

      actor ! ProcessMessage("sweep")
      eventually(errors should have size 1)
      errors.head._2 shouldBe an[IllegalStateException]
      errors.head._2.getMessage should include("Bus rejected batch topic / 1")
    }

    "send a failed pass to the plain logger when no error sink is given" in {
      val store = new InMemoryStore(Seq(message(1)))
      val produceBatch: api.ProduceBatchRequest => Future[api.ProduceBatchResponse] =
        _ => Future.failed(new RuntimeException("bus unavailable"))
      @volatile var logged = Vector.empty[String]
      val logger: String => Unit = line => synchronized { logged = logged :+ line }
      val actor = system.actorOf(Props(new FlusherActor(store, produceBatch, logger, batchSize = 100)))

      actor ! ProcessMessage("notify")
      eventually(logged.exists(line =>
        line.startsWith("Flush failed (notify)") && line.contains("bus unavailable")
      ) shouldBe true)
    }

    "skip a failing topic for the rest of the pass and deliver the topics behind it" in {
      val store = new InMemoryStore(Seq(
        message(1, "topic-a"),
        message(2, "topic-b"),
        message(3, "topic-a"),
        message(4, "topic-c"),
        message(5, "topic-b"),
      ))
      @volatile var requests = Vector.empty[api.ProduceBatchRequest]
      val produceBatch: api.ProduceBatchRequest => Future[api.ProduceBatchResponse] = request => synchronized {
        requests = requests :+ request
        if (request.topic == "topic-a") Future.failed(new RuntimeException("[29] TOPIC_AUTHORIZATION_FAILED"))
        else Future.successful(api.ProduceBatchResponse(ok = true))
      }
      @volatile var errors = Vector.empty[String]
      val errorLogger: Flusher.ErrorLogger = (line, _) => synchronized { errors = errors :+ line }
      val actor = system.actorOf(Props(new FlusherActor(store, produceBatch, _ => (), errorLogger, batchSize = 10)))

      actor ! ProcessMessage("notify")

      eventually(store.rows.map(_.id) shouldBe Seq(1L, 3L))
      eventually(errors should have size 1)
      requests.map(_.topic) shouldBe Seq("topic-a", "topic-b", "topic-c")
      requests(1).messageList.map(_.version) shouldBe Seq(2L, 5L)
      errors.head should include("for topic topic-a")
      store.deleteCalls shouldBe Seq(Seq(2L, 5L), Seq(4L))
      eventually(store.fetchFailed shouldBe Seq(Set.empty[String], Set("topic-a")))
    }

    "end the pass when only a failing topic is left" in {
      val store = new InMemoryStore(Seq(message(1, "topic-a"), message(2, "topic-a")))
      val attempts = new AtomicInteger(0)
      val produceBatch: api.ProduceBatchRequest => Future[api.ProduceBatchResponse] = _ => {
        attempts.incrementAndGet()
        Future.failed(new RuntimeException("bus unavailable"))
      }
      @volatile var errors = Vector.empty[String]
      val errorLogger: Flusher.ErrorLogger = (line, _) => synchronized { errors = errors :+ line }
      val actor = system.actorOf(Props(new FlusherActor(store, produceBatch, _ => (), errorLogger, batchSize = 1)))

      actor ! ProcessMessage("notify")
      eventually(errors should have size 1)
      Thread.sleep(200)
      attempts.get() shouldBe 1
      store.rows.map(_.id) shouldBe Seq(1L, 2L)
    }

    "report a failing topic at most once per interval and count the suppressed reports" in {
      val store = new InMemoryStore(Seq(message(1, "topic-a"), message(2, "topic-b")))
      val attempts = new AtomicInteger(0)
      val produceBatch: api.ProduceBatchRequest => Future[api.ProduceBatchResponse] = _ => {
        attempts.incrementAndGet()
        Future.failed(new RuntimeException("bus unavailable"))
      }
      val clock = new java.util.concurrent.atomic.AtomicLong(0L)
      @volatile var errors = Vector.empty[String]
      val errorLogger: Flusher.ErrorLogger = (line, _) => synchronized { errors = errors :+ line }
      val actor = system.actorOf(Props(new FlusherActor(
        store, produceBatch, _ => (), errorLogger, 100, 1.minute, () => clock.get(),
      )))

      // Both topics fail on every pass: one report per topic for the first pass.
      actor ! ProcessMessage("notify")
      eventually(attempts.get() shouldBe 2)
      eventually(errors should have size 2)
      // Two more passes within the minute are suppressed.
      clock.addAndGet(30.seconds.toNanos)
      actor ! ProcessMessage("notify")
      eventually(attempts.get() shouldBe 4)
      actor ! ProcessMessage("notify")
      eventually(attempts.get() shouldBe 6)
      errors should have size 2
      // After the interval the next report carries the suppressed count.
      clock.addAndGet(30.seconds.toNanos)
      actor ! ProcessMessage("notify")
      eventually(errors should have size 4)
      errors.drop(2).foreach(_ should include("(2 more suppressed since the previous report)"))
      errors.drop(2).map(_.contains("topic-a")) should contain(true)
      errors.drop(2).map(_.contains("topic-b")) should contain(true)
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

    "keep the batch when the bus does not answer in time and send it on a later pass" in {
      val bus = new HangingBus
      try {
        val store = new InMemoryStore(Seq(message(1), message(2)))
        @volatile var logged = Vector.empty[String]
        val logger: String => Unit = line => synchronized { logged = logged :+ line }
        val produceBatch: api.ProduceBatchRequest => Future[api.ProduceBatchResponse] =
          Producer.produceBatch(bus.stub, _, 300.millis)(Flusher.ec)
        val actor = system.actorOf(Props(new FlusherActor(store, produceBatch, logger, batchSize = 100)))

        actor ! ProcessMessage("notify")
        eventually(logged.exists(_.contains("ProduceTimeoutException")) shouldBe true)
        store.rows.map(_.id) shouldBe Seq(1L, 2L)
        store.deleteCalls shouldBe empty

        bus.hanging = false
        actor ! ProcessMessage("sweep")
        eventually(store.rows shouldBe empty)
        store.deleteCalls shouldBe Seq(Seq(1L, 2L))
        bus.batchCalls.get() shouldBe 2
      } finally bus.close()
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
