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

  /** Publishes one message with the [[defaultProduceTimeout]] deadline. */
  def produce(
    grpcClient: RedbusServiceGrpc.RedbusServiceStub,
    topic: String,
    message: Array[Byte],
    options: producer.Option.Fn*,
  )(implicit ec: ExecutionContext): Future[Boolean] =
    produceWithTimeout(grpcClient, defaultProduceTimeout, topic, message, options: _*)

  /**
   * Publishes one message; the future fails with [[ProduceTimeoutException]] when the bus does not
   * answer within `timeout`.
   */
  def produceWithTimeout(
    grpcClient: RedbusServiceGrpc.RedbusServiceStub,
    timeout: FiniteDuration,
    topic: String,
    message: Array[Byte],
    options: producer.Option.Fn*,
  )(implicit ec: ExecutionContext): Future[Boolean] = {
    val req = prepareRequest(topic, message, options: _*)
    withDeadline(grpcClient, topic, timeout)(_.produce(req)).map(_.ok)
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

  def produceDba(
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