package sergiusd.redbus

import org.apache.pekko.actor.ActorSystem
import org.scalatest.BeforeAndAfterAll
import org.scalatest.matchers.should.Matchers
import org.scalatest.wordspec.AnyWordSpec
import sergiusd.redbus.api.ConsumeRequest.{Result => Payload}
import sergiusd.redbus.consumer.{InboxMode, InboxProcessing, Model}
import slick.dbio.SuccessAction
import slick.jdbc.PostgresProfile

import scala.concurrent.duration._
import scala.concurrent.{Await, ExecutionContext, Future}

class ProtoClientSpec extends AnyWordSpec with Matchers with org.scalatest.LoneElement with BeforeAndAfterAll {

  implicit private val ec: ExecutionContext = ExecutionContext.global
  implicit private val system: ActorSystem = ActorSystem("ProtoClientSpec")

  // A database of an application's own PostgresProfile subclass: must be accepted without a cast.
  // Never connected; the client only hands it over.
  private val appDb: PostgresDriver.api.Database =
    PostgresDriver.api.Database.forURL("jdbc:postgresql://localhost/unused", driver = "org.postgresql.Driver")

  override def afterAll(): Unit = {
    appDb.close()
    Await.ready(system.terminate(), 10.seconds)
  }

  private def await[T](f: Future[T]): T = Await.result(f, 5.seconds)

  private class FakeTransport extends ProtoClient.Transport {
    @volatile var produced: Vector[(String, Array[Byte])] = Vector.empty
    @volatile var consumed: Vector[(String, String, Model.Processor, Seq[consumer.Option.Fn])] = Vector.empty
    @volatile var flusherStarts: Vector[Int] = Vector.empty

    override def produce(topic: String, message: Array[Byte], options: producer.Option.Fn*): Future[Boolean] = {
      produced = produced :+ (topic -> message)
      Future.successful(true)
    }

    override def consume(
      topic: String,
      group: String,
      processor: Model.Processor,
      addStopHook: Model.StopHook,
      options: consumer.Option.Fn*,
    ): Future[Unit] = {
      consumed = consumed :+ ((topic, group, processor, options))
      Future.unit
    }

    override def startFlusher(db: PostgresProfile.backend.Database, batchSize: Int)(implicit as: ActorSystem): Unit =
      flusherStarts = flusherStarts :+ batchSize
  }

  private def settings(producer: Boolean, consumer: Boolean) =
    RedbusSettings("localhost", 50005, producerEnabled = producer, consumerEnabled = consumer, outboxBatchSize = 7)

  private def client(s: RedbusSettings, transport: => ProtoClient.Transport, log: ProtoClient.Log = ProtoClient.Log()) =
    new ProtoClient(s, appDb, _ => (), log, transport)

  private val message = Payload(id = "1", message = "payload")

  "ProtoClient with the bus disabled" should {
    "be a no-op without ever creating a transport" in {
      val c = client(settings(producer = false, consumer = false), fail("transport must not be created"))

      await(c.produceProto("topic", message)) shouldBe false
      c.produceProtoDba("topic", message) shouldBe SuccessAction(0)
      await(c.consumeProto[Payload]("topic", "group", InboxMode.OnlyOnce)((_, _) => fail("processor"))) shouldBe (())
      c.startFlusher()
    }

    "keep a disabled side a no-op while the other side works" in {
      val transport = new FakeTransport
      val consumerOnly = client(settings(producer = false, consumer = true), transport)

      await(consumerOnly.produceProto("topic", message)) shouldBe false
      consumerOnly.produceProtoDba("topic", message) shouldBe SuccessAction(0)
      consumerOnly.startFlusher()
      transport.produced shouldBe empty
      transport.flusherStarts shouldBe empty

      val producerOnly = client(settings(producer = true, consumer = false), transport)
      await(producerOnly.consumeProto[Payload]("topic", "group", InboxMode.OnlyOnce)((_, _) => Future.unit))
      transport.consumed shouldBe empty
      await(producerOnly.produceProto("topic", message)) shouldBe true
      transport.produced.map(_._1) shouldBe Vector("topic")
      producerOnly.produceProtoDba("topic", message) should not be SuccessAction(0)
    }
  }

  "ProtoClient.startFlusher" should {
    "start the flusher once with the configured batch size" in {
      val transport = new FakeTransport
      val c = client(settings(producer = true, consumer = true), transport)

      c.startFlusher()
      c.startFlusher()

      transport.flusherStarts shouldBe Vector(7)
    }
  }

  "ProtoClient.consumeProto" should {
    val init = Model.Listener(consumeTimeout = 1.second, batchSize = 1, unavailableTimeout = 1.second)

    def listenerFor(inbox: InboxMode, extra: consumer.Option.Fn*): Model.Listener = {
      val transport = new FakeTransport
      val c = client(settings(producer = false, consumer = true), transport)
      await(c.consumeProto[Payload]("topic", "group", inbox, extra: _*)((_, _) => Future.unit))
      val (topic, group, _, options) = transport.consumed.loneElement
      (topic, group) shouldBe ("topic" -> "group")
      options.foldLeft(init)((x, fn) => fn(x))
    }

    "map every inbox mode onto the consumer options" in {
      val onlyOnce = listenerFor(InboxMode.OnlyOnce)
      onlyOnce.checkEventProcessedDatabase shouldBe defined
      onlyOnce.transactionalInbox shouldBe false

      val transactional = listenerFor(InboxMode.Transactional)
      transactional.checkEventProcessedDatabase shouldBe defined
      transactional.transactionalInbox shouldBe true

      val disabled = listenerFor(InboxMode.Disabled)
      disabled.checkEventProcessedDatabase shouldBe None
      disabled.transactionalInbox shouldBe false
    }

    "apply the caller's options after the inbox" in {
      listenerFor(InboxMode.OnlyOnce, consumer.Option.WithBatchSize(5)).batchSize shouldBe 5
    }
  }

  "ProtoClient.decoding" should {
    val meta = Model.MessageMeta()

    "hand a decoded message to the processor" in {
      @volatile var received: Option[Payload] = None
      val processor = ProtoClient.decoding[Payload]("topic", "group", ProtoClient.Log()) { (m, _) =>
        received = Some(m)
        Future.unit
      }

      await(processor(message.toByteArray, meta))
      received shouldBe Some(message)
    }

    "log and acknowledge a malformed payload without calling the processor" in {
      @volatile var logged: Vector[String] = Vector.empty
      val log = ProtoClient.Log(error = (msg, _) => logged = logged :+ msg)
      val processor = ProtoClient.decoding[Payload]("topic", "group", log)((_, _) => fail("processor"))

      await(processor(Array(0xff.toByte), meta)) shouldBe (())
      logged shouldBe Vector("redbus.receive.invalid: topic=topic group=group payloadBytes=1 action=drop")
    }

    "turn a synchronous processor exception into a failure of that message only" in {
      val processor = ProtoClient.decoding[Payload]("topic", "group", ProtoClient.Log()) { (m, _) =>
        if (m.message == "boom") throw new IllegalStateException("sync failure") else Future.unit
      }
      def process(payload: String) = await(InboxProcessing.process(
        store = None,
        transactional = false,
        group = "group",
        topic = "topic",
        idempotencyKey = payload,
        data = Payload(message = payload).toByteArray,
        meta = meta,
        processor = processor,
        log = _ => (),
      ))

      process("boom").left.map(_.getMessage) shouldBe Left("sync failure")
      process("fine") shouldBe Right(())
    }
  }

  "RedbusSettings.fromConfig" should {
    "read the keys and default the outbox batch size" in {
      val base = com.typesafe.config.ConfigFactory.parseString(
        "host = bus, port = 50005, producerEnabled = yes, consumerEnabled = false"
      )
      RedbusSettings.fromConfig(base) shouldBe RedbusSettings("bus", 50005, producerEnabled = true, consumerEnabled = false)
      RedbusSettings.fromConfig(com.typesafe.config.ConfigFactory.parseString("outboxBatchSize = 20").withFallback(base))
        .outboxBatchSize shouldBe 20
    }
  }

  "consumer.Option.WithInbox" should {
    "accept an application profile's database without a cast" in {
      val listener = consumer.Option.WithInbox(appDb, InboxMode.Transactional)(
        Model.Listener(consumeTimeout = 1.second, batchSize = 1, unavailableTimeout = 1.second)
      )
      listener.transactionalInbox shouldBe true
    }
  }
}
