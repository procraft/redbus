package sergiusd.redbus.producer

import com.google.protobuf.ByteString
import sergiusd.redbus.api.{ProduceRequest, RedbusServiceGrpc}
import sergiusd.redbus.producer

import scala.concurrent.{ExecutionContext, Future}

object Producer {

  def produce(
    grpcClient: RedbusServiceGrpc.RedbusServiceStub,
    topic: String,
    message: Array[Byte],
    options: producer.Option.Fn*,
  )(implicit ec: ExecutionContext): Future[Boolean] = {
    val req = options.foldLeft(ProduceRequest(topic = topic, message = ByteString.copyFrom(message)))((x, fn) => fn(x))
    grpcClient.produce(req).map(_.ok)
  }

}