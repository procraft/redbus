package sergiusd.redbus.producer

import com.google.protobuf.ByteString
import io.grpc.Status
import sergiusd.redbus.api.{ProduceBatchRequest, ProduceBatchResponse, ProduceRequest, RedbusServiceGrpc}
import sergiusd.redbus.producer
import slick.dbio.{DBIOAction, Effect}
import slick.jdbc.PostgresProfile.api._

import java.time.ZonedDateTime
import java.util.UUID
import java.util.concurrent.TimeUnit
import scala.concurrent.duration._
import scala.concurrent.{ExecutionContext, Future}

object Producer {

  /** Default deadline of one direct `Produce` call. */
  val defaultProduceTimeout: FiniteDuration = 30.seconds
  /** Default deadline of one `ProduceBatch` call (the outbox flusher). */
  val defaultProduceBatchTimeout: FiniteDuration = 30.seconds
  /**
   * Default limit of one message payload, in bytes (256 KiB). Kafka rejects records above its
   * `message.max.bytes` (about 1 MB by default) and the bus's gRPC server a request above 4 MiB, so a
   * larger payload could never be delivered and, in the outbox, would hold back its topic.
   */
  val defaultMaxMessageBytes: Int = 256 * 1024

  /** Publishes one message with the [[defaultProduceTimeout]] deadline. */
  def produce(
    grpcClient: RedbusServiceGrpc.RedbusServiceStub,
    topic: String,
    message: Array[Byte],
    options: producer.Option.Fn*,
  )(implicit ec: ExecutionContext): Future[Boolean] =
    produceWithTimeout(grpcClient, defaultProduceTimeout, topic, message, options: _*)

  /**
   * Publishes one message with the [[defaultMaxMessageBytes]] limit; the future fails with
   * [[ProduceTimeoutException]] when the bus does not answer within `timeout`.
   */
  def produceWithTimeout(
    grpcClient: RedbusServiceGrpc.RedbusServiceStub,
    timeout: FiniteDuration,
    topic: String,
    message: Array[Byte],
    options: producer.Option.Fn*,
  )(implicit ec: ExecutionContext): Future[Boolean] =
    produceWithLimits(grpcClient, timeout, defaultMaxMessageBytes, topic, message, options: _*)

  /**
   * Publishes one message. The future fails with [[MessageTooLargeException]] without calling the bus
   * when the payload is longer than `maxMessageBytes`, and with [[ProduceTimeoutException]] when the
   * bus does not answer within `timeout`.
   */
  def produceWithLimits(
    grpcClient: RedbusServiceGrpc.RedbusServiceStub,
    timeout: FiniteDuration,
    maxMessageBytes: Int,
    topic: String,
    message: Array[Byte],
    options: producer.Option.Fn*,
  )(implicit ec: ExecutionContext): Future[Boolean] =
    tooLarge(topic, message, maxMessageBytes) match {
      case Some(e) => Future.failed(e)
      case None =>
        val req = prepareRequest(topic, message, options: _*)
        withDeadline(grpcClient, topic, timeout)(_.produce(req)).map(_.ok)
    }

  /**
   * The error for a payload longer than `maxMessageBytes`, `None` when it fits. The size is
   * `message.length`: the payload bytes, without topic, key or headers.
   */
  def tooLarge(topic: String, message: Array[Byte], maxMessageBytes: Int): scala.Option[MessageTooLargeException] = {
    require(maxMessageBytes > 0, "maxMessageBytes must be positive")
    if (message.length > maxMessageBytes) Some(MessageTooLargeException(topic, message.length, maxMessageBytes))
    else None
  }

  /**
   * Publishes a same-topic batch with one confirmed request; the future fails with
   * [[ProduceTimeoutException]] when the bus does not answer within `timeout`.
   */
  def produceBatch(
    grpcClient: RedbusServiceGrpc.RedbusServiceStub,
    request: ProduceBatchRequest,
    timeout: FiniteDuration = defaultProduceBatchTimeout,
  )(implicit ec: ExecutionContext): Future[ProduceBatchResponse] =
    withDeadline(grpcClient, request.topic, timeout)(_.produceBatch(request))

  // The deadline is absolute, so it is set on a per-call copy of the stub.
  private def withDeadline[T](
    grpcClient: RedbusServiceGrpc.RedbusServiceStub,
    topic: String,
    timeout: FiniteDuration,
  )(call: RedbusServiceGrpc.RedbusServiceStub => Future[T])(implicit ec: ExecutionContext): Future[T] = {
    require(timeout > Duration.Zero, "produce timeout must be positive")
    call(grpcClient.withDeadlineAfter(timeout.toMillis, TimeUnit.MILLISECONDS)).recoverWith {
      case e if Status.fromThrowable(e).getCode == Status.Code.DEADLINE_EXCEEDED =>
        Future.failed(ProduceTimeoutException(topic, timeout, e))
    }
  }

  /** Transactional outbox write with the [[defaultMaxMessageBytes]] limit; see [[produceDbaWithLimit]]. */
  def produceDba(
    topic: String,
    message: Array[Byte],
    options: producer.Option.Fn*,
  ): DBIOAction[Int, NoStream, Effect.Write] =
    produceDbaWithLimit(topic, message, defaultMaxMessageBytes, options: _*)

  /**
   * Transactional outbox: inserts the message into `redbus_outbox` inside the caller's transaction.
   * A payload longer than `maxMessageBytes` gives `DBIO.failed(MessageTooLargeException)`: no row is
   * written and the caller's transaction rolls back.
   */
  def produceDbaWithLimit(
    topic: String,
    message: Array[Byte],
    maxMessageBytes: Int,
    options: producer.Option.Fn*,
  ): DBIOAction[Int, NoStream, Effect.Write] = tooLarge(topic, message, maxMessageBytes) match {
    case Some(e) => DBIO.failed(e)
    case None => insert(topic, message, options: _*)
  }

  private def insert(
    topic: String,
    message: Array[Byte],
    options: producer.Option.Fn*,
  ): DBIOAction[Int, NoStream, Effect.Write] = {
    val req = prepareRequest(topic, message, options: _*)
    PublishingMessages += PublishingMessage(
      req.topic,
      message,
      PublishingMessage.Options(
        if (req.key.nonEmpty) Some(req.key) else None,
        if (req.version != 0) Some(req.version) else None,
        if (req.idempotencyKey.nonEmpty) Some(req.idempotencyKey) else None,
        if (req.timestamp.nonEmpty) Some(req.timestamp) else None,
      ),
    )
  }

  private def prepareRequest(
    topic: String,
    message: Array[Byte],
    options: producer.Option.Fn*,
   ): ProduceRequest = {
    options.foldLeft(ProduceRequest(
      topic = topic,
      message = ByteString.copyFrom(message),
      idempotencyKey = UUID.randomUUID().toString,
      timestamp = ZonedDateTime.now.toOffsetDateTime.toString,
    ))((x, fn) => fn(x))
  }
}