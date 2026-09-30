package sergiusd.redbus.producer

import io.grpc.stub.StreamObserver
import io.grpc.{ManagedChannel, ManagedChannelBuilder, Server, ServerBuilder}
import sergiusd.redbus.api

import java.util.concurrent.atomic.AtomicInteger
import scala.concurrent.{ExecutionContext, Future, Promise}

/** A local bus whose produce calls never answer while `hanging` is set. */
final class HangingBus {
  @volatile var hanging: Boolean = true
  val produceCalls = new AtomicInteger(0)
  val batchCalls = new AtomicInteger(0)

  private val service = new api.RedbusServiceGrpc.RedbusService {
    override def produce(request: api.ProduceRequest): Future[api.ProduceResponse] = {
      produceCalls.incrementAndGet()
      if (hanging) Promise[api.ProduceResponse]().future else Future.successful(api.ProduceResponse(ok = true))
    }

    override def produceBatch(request: api.ProduceBatchRequest): Future[api.ProduceBatchResponse] = {
      batchCalls.incrementAndGet()
      if (hanging) Promise[api.ProduceBatchResponse]().future
      else Future.successful(api.ProduceBatchResponse(ok = true))
    }

    override def consume(responseObserver: StreamObserver[api.ConsumeResponse]): StreamObserver[api.ConsumeRequest] =
      throw new UnsupportedOperationException("consume")
  }

  private val server: Server = ServerBuilder
    .forPort(0)
    .addService(api.RedbusServiceGrpc.bindService(service, ExecutionContext.global))
    .build()
    .start()

  private val channel: ManagedChannel =
    ManagedChannelBuilder.forAddress("localhost", server.getPort).usePlaintext().build()

  val stub: api.RedbusServiceGrpc.RedbusServiceStub = api.RedbusServiceGrpc.stub(channel)

  def close(): Unit = {
    channel.shutdownNow()
    server.shutdownNow()
  }
}
