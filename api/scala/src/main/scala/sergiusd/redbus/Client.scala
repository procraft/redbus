package sergiusd.redbus

import akka.actor.ActorSystem
import sergiusd.redbus.api.RedbusServiceGrpc

class Client(host: String, port: Int) {

  private lazy val grpcClientFactory = new GrpcClientFactory(ActorSystem.create())
  private lazy val grpc = grpcClientFactory.get(
    host, port, RedbusServiceGrpc.stub,
  )



}