package sergiusd.redbus

import akka.actor.ActorSystem
import sergiusd.redbus.api._
import sergiusd.redbus.producer.Flusher

import scala.concurrent.{ExecutionContext, Future}

case class Client(
  host: String,
  port: Int,
  logger: String => Unit = _ => (),
)(implicit ec: ExecutionContext) {

  private lazy val grpcClientFactory = new GrpcClientFactory(ActorSystem.create())
  private lazy val grpc = grpcClientFactory.get(host, port, RedbusServiceGrpc.stub)

  def produce(
    topic: String,
    message: Array[Byte],
    options: producer.Option.Fn*,
  ): Future[Boolean] = {
    producer.Producer.produce(grpc, topic, message, options: _*)
  }

  def startProducerDbaFlusher(
    db: slick.jdbc.PostgresProfile.backend.Database,
  )(implicit as: ActorSystem): Unit = {
    Flusher.start(db, grpc.produce, logger)
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