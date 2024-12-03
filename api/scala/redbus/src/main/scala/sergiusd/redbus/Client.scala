package sergiusd.redbus

import akka.actor.ActorSystem
import com.google.protobuf.ByteString
import sergiusd.redbus.api._

import scala.concurrent.{ExecutionContext, Future}

case class Client(host: String, port: Int)(implicit ec: ExecutionContext) {

  private lazy val grpcClientFactory = new GrpcClientFactory(ActorSystem.create())
  private lazy val grpc = grpcClientFactory.get(host, port, RedbusServiceGrpc.stub)

  def produce(topic: String, message: Array[Byte], key: String = ""): Future[Unit] = {
    grpc.produce(ProduceRequest(topic, key, ByteString.copyFrom(message))).map(_ => ())
  }

  def consume(
    topic: String,
    group: String,
    processor: consumer.Model.Processor,
    addStopHook: consumer.Model.StopHook,
  ): Future[Unit] = {
    new consumer.Consumer(grpc, s"$host:$port", topic, group, processor, addStopHook).consume()
  }

  def close(): Unit = {
    grpcClientFactory.shutdown()
  }

}