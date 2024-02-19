package sergiusd.redbus

import akka.actor.ActorSystem
import com.google.protobuf.ByteString
import sergiusd.redbus.api._

case class Client(host: String, port: Int) {

  private lazy val grpcClientFactory = new GrpcClientFactory(ActorSystem.create())
  private lazy val grpc = grpcClientFactory.get(host, port, RedbusServiceGrpc.stub)

  def produce(topic: String, message: Array[Byte], key: String = ""): Unit = {
    grpc.produce(ProduceRequest(topic, key, ByteString.copyFrom(message)))
  }

  def consume(topic: String, group: String, processor: Array[Byte] => Either[String, Unit]): Unit = {
    println("Consume")
    //grpc.consume()
  }

}