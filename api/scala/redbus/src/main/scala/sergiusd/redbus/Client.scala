package sergiusd.redbus

import org.apache.pekko.actor.ActorSystem
import sergiusd.redbus.api._
import sergiusd.redbus.producer.{Flusher, Producer}

import scala.concurrent.duration.FiniteDuration
import scala.concurrent.{ExecutionContext, Future}

/**
 * @param produceTimeout      deadline of one direct `produce` call (default 30 s)
 * @param produceBatchTimeout deadline of one batch request of the outbox flusher (default 30 s)
 */
case class Client(
  host: String,
  port: Int,
  logger: String => Unit = _ => (),
  produceTimeout: FiniteDuration = Producer.defaultProduceTimeout,
  produceBatchTimeout: FiniteDuration = Producer.defaultProduceBatchTimeout,
)(implicit ec: ExecutionContext) {
  require(produceTimeout.length > 0, "produceTimeout must be positive")
  require(produceBatchTimeout.length > 0, "produceBatchTimeout must be positive")

  private lazy val grpcClientFactory = new GrpcClientFactory(ActorSystem.create())
  private lazy val grpc = grpcClientFactory.get(host, port, RedbusServiceGrpc.stub)

  /**
   * Publishes directly over gRPC. Fails with `producer.ProduceTimeoutException` when the bus does
   * not answer within `produceTimeout`; the outcome of such a call is unknown.
   */
  def produce(
    topic: String,
    message: Array[Byte],
    options: producer.Option.Fn*,
  ): Future[Boolean] = {
    Producer.produceWithTimeout(grpc, produceTimeout, topic, message, options: _*)
  }

  /**
   * Starts the transactional-outbox flusher for rows written with `producer.Producer.produceDba`.
   * Besides reacting to `pg_notify`, it sweeps `redbus_outbox` every `sweepInterval` (default 30 s)
   * and publishes ordered, bounded batches (default 100 rows). Each batch request is bounded by
   * `produceBatchTimeout`; a timed-out batch stays in the outbox and is sent again on the next pass.
   */
  def startProducerDbaFlusher(
    db: slick.jdbc.PostgresProfile.backend.Database,
    sweepInterval: FiniteDuration = Flusher.defaultSweepInterval,
    batchSize: Int = Flusher.defaultBatchSize,
  )(implicit as: ActorSystem): Unit = {
    Flusher.start(db, Producer.produceBatch(grpc, _, produceBatchTimeout), logger, sweepInterval, batchSize)
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
