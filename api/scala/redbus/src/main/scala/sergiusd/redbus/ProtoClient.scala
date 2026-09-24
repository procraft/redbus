package sergiusd.redbus

import org.apache.pekko.actor.ActorSystem
import scalapb.{GeneratedMessage, GeneratedMessageCompanion}
import slick.dbio.{DBIOAction, Effect, NoStream}
import slick.jdbc.{JdbcBackend, PostgresProfile}

import java.util.concurrent.atomic.AtomicBoolean
import scala.concurrent.{ExecutionContext, Future}
import scala.util.control.NonFatal

/**
 * ScalaPB-typed bus client configured by [[RedbusSettings]]; the recommended entry point for a
 * service. A disabled side is a no-op: `produceProto` answers `false`, `produceProtoDba` writes no
 * row, `consumeProto` completes at once and `startFlusher` starts nothing.
 *
 * `database` holds the service's `redbus_inbox` / `redbus_outbox` tables; any Slick `JdbcProfile`
 * database is accepted without a cast. `addStopHook` registers consumer shutdown, e.g. Play's
 * `ApplicationLifecycle.addStopHook`.
 */
final class ProtoClient private[redbus] (
  val settings: RedbusSettings,
  database: JdbcBackend#JdbcDatabaseDef,
  addStopHook: consumer.Model.StopHook,
  log: ProtoClient.Log,
  transport: => ProtoClient.Transport,
)(implicit ec: ExecutionContext) {

  private lazy val bus: scala.Option[ProtoClient.Transport] = if (settings.enabled) {
    log.info(s"redbus: connect to service on ${settings.host}:${settings.port}")
    Some(transport)
  } else None

  private val flusherStarted = new AtomicBoolean(false)

  /** Publishes directly over gRPC; `false` when the producer is disabled or the bus rejected it. */
  def produceProto[T <: GeneratedMessage](topic: String, message: T, options: producer.Option.Fn*): Future[Boolean] =
    bus match {
      case Some(b) if settings.producerEnabled => b.produce(topic, message.toByteArray, options: _*)
      case _ => Future.successful(false)
    }

  /**
   * Transactional outbox: inserts the message into `redbus_outbox` inside the caller's transaction;
   * [[startFlusher]] delivers it. Writes nothing (0 rows) when the producer is disabled.
   */
  def produceProtoDba[T <: GeneratedMessage](
    topic: String,
    message: T,
    options: producer.Option.Fn*,
  ): DBIOAction[Int, NoStream, Effect.Write] =
    if (settings.producerEnabled) producer.Producer.produceDba(topic, message.toByteArray, options: _*)
    else DBIOAction.successful(0)

  /**
   * Starts the outbox flusher once per client; later calls do nothing. The flusher actor has a fixed
   * name, so start it from a single client per actor system.
   */
  def startFlusher()(implicit as: ActorSystem): Unit = bus match {
    case Some(b) if settings.producerEnabled && flusherStarted.compareAndSet(false, true) =>
      log.info(s"redbus: start outbox flusher, batch size = ${settings.outboxBatchSize}")
      b.startFlusher(JdbcDatabases.postgres(database), settings.outboxBatchSize)
    case _ => ()
  }

  /**
   * Consumes `topic` as `group`, decoding each payload as `T`.
   *
   * A payload that is not a valid `T` is logged and acknowledged: a retry cannot fix it. An exception
   * the processor throws synchronously fails only that message, like a failed future.
   *
   * @param inbox   inbox dedup mode; the inbox lives in the client's `database`
   * @param options further consumer options (repeat strategy, batch size, timeouts)
   */
  def consumeProto[T <: GeneratedMessage](
    topic: String,
    group: String,
    inbox: consumer.InboxMode,
    options: consumer.Option.Fn*,
  )(processor: (T, consumer.Model.MessageMeta) => Future[Unit])(
    implicit companion: GeneratedMessageCompanion[T]
  ): Future[Unit] = bus match {
    case Some(b) if settings.consumerEnabled =>
      log.info(s"redbus: consume topic $topic, group $group, inbox $inbox")
      b.consume(
        topic,
        group,
        ProtoClient.decoding(topic, group, log)(processor),
        addStopHook,
        consumer.Option.WithInbox(database, inbox) +: options: _*,
      )
    case _ =>
      log.info(s"redbus: consumer for topic $topic, group $group is disabled")
      Future.unit
  }
}

object ProtoClient {

  /**
   * @param logger receives the SDK's own diagnostic messages (debug level)
   */
  def apply(
    settings: RedbusSettings,
    database: JdbcBackend#JdbcDatabaseDef,
    addStopHook: consumer.Model.StopHook,
    log: Log = Log(),
  )(implicit ec: ExecutionContext): ProtoClient =
    new ProtoClient(settings, database, addStopHook, log, new ClientTransport(Client(settings.host, settings.port, log.debug)))

  /** Log sinks of the client; each defaults to discarding. */
  final case class Log(
    debug: String => Unit = _ => (),
    info: String => Unit = _ => (),
    error: (String, Throwable) => Unit = (_, _) => (),
  )

  /** What [[ProtoClient]] needs from the bus; replaced in unit tests. */
  private[redbus] trait Transport {
    def produce(topic: String, message: Array[Byte], options: producer.Option.Fn*): Future[Boolean]
    def consume(
      topic: String,
      group: String,
      processor: consumer.Model.Processor,
      addStopHook: consumer.Model.StopHook,
      options: consumer.Option.Fn*,
    ): Future[Unit]
    def startFlusher(db: PostgresProfile.backend.Database, batchSize: Int)(implicit as: ActorSystem): Unit
  }

  private final class ClientTransport(client: Client) extends Transport {
    override def produce(topic: String, message: Array[Byte], options: producer.Option.Fn*): Future[Boolean] =
      client.produce(topic, message, options: _*)

    override def consume(
      topic: String,
      group: String,
      processor: consumer.Model.Processor,
      addStopHook: consumer.Model.StopHook,
      options: consumer.Option.Fn*,
    ): Future[Unit] = client.consume(topic, group, processor, addStopHook, options: _*)

    override def startFlusher(db: PostgresProfile.backend.Database, batchSize: Int)(implicit as: ActorSystem): Unit =
      client.startProducerDbaFlusher(db, batchSize = batchSize)
  }

  /** Wraps a typed processor into the SDK's byte processor (see [[ProtoClient.consumeProto]]). */
  private[redbus] def decoding[T <: GeneratedMessage](topic: String, group: String, log: Log)(
    processor: (T, consumer.Model.MessageMeta) => Future[Unit]
  )(implicit companion: GeneratedMessageCompanion[T]): consumer.Model.Processor = { (data, meta) =>
    val parsed =
      try Right(companion.parseFrom(data))
      catch { case NonFatal(e) => Left(e) }
    parsed match {
      case Left(e) =>
        log.error(s"redbus.receive.invalid: topic=$topic group=$group payloadBytes=${data.length} action=drop", e)
        Future.unit
      case Right(message) =>
        try processor(message, meta)
        catch { case NonFatal(e) => Future.failed(e) }
    }
  }
}
