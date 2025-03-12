package sergiusd.redbus.producer

import com.google.protobuf.ByteString
import sergiusd.redbus.api.{ProduceRequest, RedbusServiceGrpc}
import sergiusd.redbus.producer

import java.time.ZonedDateTime
import java.util.UUID
import scala.concurrent.{ExecutionContext, Future}

object Producer {

  def produce(
    grpcClient: RedbusServiceGrpc.RedbusServiceStub,
    topic: String,
    message: Array[Byte],
    options: producer.Option.Fn*,
  )(implicit ec: ExecutionContext): Future[Boolean] = {
    val req = options.foldLeft(ProduceRequest(
      topic = topic,
      message = ByteString.copyFrom(message),
      idempotencyKey = UUID.randomUUID().toString,
      timestamp = ZonedDateTime.now.toOffsetDateTime.toString,
    ))((x, fn) => fn(x))
    grpcClient.produce(req).map(_.ok)
  }

}