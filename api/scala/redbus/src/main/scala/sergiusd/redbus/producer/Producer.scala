package sergiusd.redbus.producer

import akka.actor.ActorSystem
import com.google.protobuf.ByteString
import sergiusd.redbus.api.{ProduceRequest, RedbusServiceGrpc}
import sergiusd.redbus.producer
import slick.dbio.{DBIOAction, Effect}
import slick.jdbc.PostgresProfile.api._

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
    val req = prepareRequest(topic, message, options: _*)
    grpcClient.produce(req).map(_.ok)
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