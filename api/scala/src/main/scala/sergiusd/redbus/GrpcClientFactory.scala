package sergiusd.redbus

import akka.actor.ActorSystem
import io.grpc.stub.AbstractStub
import io.grpc.{ManagedChannel, ManagedChannelBuilder}

import scala.collection.mutable


class GrpcClientFactory(actorSystem: ActorSystem) {

  private val channels = mutable.Map[String, ManagedChannel]()

  def get[T <: AbstractStub[_]](host: String, port: Int, stubFn: ManagedChannel => T): T = {
    stubFn(getChannel(host, port))
  }

  private def getChannel(host: String, port: Int): ManagedChannel = {
    val channelId = s"$host:$port"
    // protect mutable map from multi-thread access
    this.synchronized {
      if (!channels.contains(channelId)) {
        val channel = ManagedChannelBuilder
          .forAddress(host, port)
          .usePlaintext().build
        channels += channelId -> channel
      }
      channels(channelId)
    }
  }

  actorSystem.registerOnTermination {
    System.err.println("*** shutting down gRPC client channels since actor system is shutting down")
    channels.values.foreach(_.shutdown())
  }
}
