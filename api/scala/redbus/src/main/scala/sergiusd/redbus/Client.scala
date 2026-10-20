package sergiusd.redbus

import org.apache.pekko.actor.ActorSystem
import sergiusd.redbus.api._
import sergiusd.redbus.producer.{Flusher, Producer}

import scala.concurrent.duration.FiniteDuration
import scala.concurrent.{ExecutionContext, Future}

/**
 * @param produceTimeout      deadline of one direct `produce` call (default 30 s)
 * @param produceBatchTimeout deadline of one batch request of the outbox flusher (default 30 s)
 * @param errorLogger         receives every failed outbox flush pass with its cause; when absent
 *                            the report goes to `logger`, as in releases before 0.4.7
 * @param maxMessageBytes     limit of one message payload for `produce` and `produceDba` (default
 *                            256 KiB, `Producer.defaultMaxMessageBytes`); a longer payload fails with
 *                            `producer.MessageTooLargeException`
 */
case class Client(
  host: String,
  port: Int,
  logger: String => Unit = _ => (),
  produceTimeout: FiniteDuration = Producer.defaultProduceTimeout,
  produceBatchTimeout: FiniteDuration = Producer.defaultProduceBatchTimeout,
  errorLogger: scala.Option[(String, Throwable) => Unit] = None,
  maxMessageBytes: Int = Producer.defaultMaxMessageBytes,
)(implicit ec: ExecutionContext) {
  require(produceTimeout.length > 0, "produceTimeout must be positive")
  require(produceBatchTimeout.length > 0, "produceBatchTimeout must be positive")
  require(maxMessageBytes > 0, "maxMessageBytes must be positive")

  private lazy val grpcClientFactory = new GrpcClientFactory(ActorSystem.create())
  private lazy val grpc = grpcClientFactory.get(host, port, RedbusServiceGrpc.stub)

  /**
   * Publishes directly over gRPC. Fails with `producer.MessageTooLargeException` without calling the
   * bus when the payload is longer than `maxMessageBytes`, and with
   * `producer.ProduceTimeoutException` when the bus does not answer within `produceTimeout`; the
   * outcome of a timed-out call is unknown.
   */
  def produce(
    topic: String,
    message: Array[Byte],
    options: producer.Option.Fn*,
  ): Future[Boolean] = {
    Producer.produceWithLimits(grpc, produceTimeout, maxMessageBytes, topic, message, options: _*)
  }

  /**
   * Transactional outbox write (`producer.Producer.produceDbaWithLimit`) with this client's
   * `maxMessageBytes`: a longer payload fails the action with `producer.MessageTooLargeException`,
   * so no row is written and the caller's transaction rolls back.
   */
  def produceDba(
    topic: String,
    message: Array[Byte],
    options: producer.Option.Fn*,
  ): slick.dbio.DBIOAction[Int, slick.dbio.NoStream, slick.dbio.Effect.Write] =
    Producer.produceDbaWithLimit(topic, message, maxMessageBytes, options: _*)

  /**
   * Starts the transactional-outbox flusher for rows written with `producer.Producer.produceDba`.
   * Besides reacting to `pg_notify`, it sweeps `redbus_outbox` every `sweepInterval` (default 30 s)
   * and publishes ordered, bounded batches (default 100 rows). Each batch request is bounded by
   * `produceBatchTimeout`; a timed-out batch stays in the outbox and is sent again on the next pass.
   * A failed pass is reported through `errorLogger` (or `logger` when it is absent).
   */
  def startProducerDbaFlusher(
    db: slick.jdbc.PostgresProfile.backend.Database,
    sweepInterval: FiniteDuration = Flusher.defaultSweepInterval,
    batchSize: Int = Flusher.defaultBatchSize,
  )(implicit as: ActorSystem): Unit = {
    Flusher.start(
      db,
      Producer.produceBatch(grpc, _, produceBatchTimeout),
      logger,
      sweepInterval,
      batchSize,
      errorLogger,
    )
  }

  def consume(
    topic: String,
    group: String,
    processor: consumer.Model.Processor,
    addStopHook: consumer.Model.StopHook,
    options: consumer.Option.Fn*,
  ): Future[Unit] = {
    new consumer.Consumer(
      grpc, s"$host:$port", topic, group, processor, addStopHook,
      options :+ consumer.Option.withLogger(logger): _*,
    ).consume()
  }

  def close(): Unit = {
    grpcClientFactory.shutdown()
  }

}
