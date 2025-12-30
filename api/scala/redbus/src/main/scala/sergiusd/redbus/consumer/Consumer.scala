package sergiusd.redbus.consumer

import akka.Done
import akka.actor.ActorSystem
import akka.pattern.after
import io.grpc.stub.StreamObserver
import sergiusd.redbus.api._
import sergiusd.redbus
import sergiusd.redbus.consumer.Model.MessageMeta

import java.time.ZonedDateTime
import java.util.concurrent.TimeUnit
import scala.concurrent.{ExecutionContext, Future, Promise, TimeoutException}
import scala.concurrent.duration.FiniteDuration
import scala.util.{Failure, Success, Try}
import slick.jdbc.PostgresProfile.backend.Database

class Consumer(
  grpcClient: RedbusServiceGrpc.RedbusServiceStub,
  hostPort: String,
  topic: String,
  group: String,
  processor: Model.Processor,
  addStopHook: Model.StopHook,
  options: Option.Fn*,
) {

  implicit private val actorSystem: ActorSystem = ActorSystem("ConsumerActorSystem")
  implicit private val ec: ExecutionContext = actorSystem.dispatcher

  private var isConnected = false
  private var isRunning = true

  private val promise = Promise[Done]()
  private var attempt = 0
  private val listener = {
    val init = Model.Listener(
      consumeTimeout = new FiniteDuration(60, TimeUnit.SECONDS),
      repeatStrategy = None,
      batchSize = 1,
      unavailableTimeout = new FiniteDuration(60, TimeUnit.SECONDS),
    )
    options.foldLeft(init)((x, fn) => fn(x))
  }
  private var requestObserver: Option[StreamObserver[ConsumeRequest]] = None
  private val connectRequest = ConsumeRequest(
    connect = Some(
      ConsumeRequest.Connect(
        id = s"${ProcessHandle.current().pid()}-${System.currentTimeMillis()}",
        topic = topic,
        group = group,
        repeatStrategy = listener.repeatStrategy.map(_.toPB),
        batchSize = listener.batchSize,
      )
    )
  )

  def consume(): Future[Unit] = {

    listener.logger("Start consumer")
    connectAndServe()

    addStopHook { () =>
      Future {
        isRunning = false
        promise.trySuccess(Done)
        listener.logger("Consumer shutting down")
      }
    }

    promise.future.map(_ => ())
  }

  private def connectAndServe(): Unit = {
    if (!isRunning) return

    val responseObserver = new StreamObserver[ConsumeResponse] {

      override def onNext(response: ConsumeResponse): Unit = {
        if (!isConnected) {
          response.connect match {
            case Some(connect) => precessConnect(connect)
            case _ => reconnect(s"Connect response is empty on connect request: $response")
          }
        } else {
          processMessageList(response.messageList).onComplete {
            case Success(_) => ()
            case Failure(e) => reconnect(s"Error on process message list: ${e.getMessage}")
          }
        }
      }

      override def onError(e: Throwable): Unit = {
        if (isRunning) {
          reconnect(e.getMessage)
        } else {
          promise.failure(e)
        }
      }

      override def onCompleted(): Unit = {
        listener.logger("Stream completed")
        promise.success(Done)
      }
    }

    requestObserver = Some(grpcClient.consume(responseObserver))

    sendRequest(connectRequest)
  }

  private def reconnect(error: String): Unit = {
    requestObserver = None
    attempt += 1
    listener.logger(s"Connect to $hostPort error: $error, attempt $attempt, ${listener.unavailableTimeout} waiting...")
    runWithPause(listener.unavailableTimeout)(connectAndServe())
  }

  private def sendRequest(request: ConsumeRequest): Unit = {
    try {
      listener.logger("Send connect request")
      requestObserver.foreach(_.onNext(request))
    } catch {
      case e: Throwable => reconnect(e.getMessage)
    }
  }

  private def precessConnect(response: ConsumeResponse.Connect): Unit = {
    if (response.ok) {
      attempt = 0
      isConnected = true
      listener.logger(s"Connect to $hostPort, id = ${connectRequest.connect.map(_.id).getOrElse("?")}")
    } else {
      reconnect(response.message)
    }
  }

  private def processMessageList(messageList: Seq[ConsumeResponse.Message]): Future[Unit] = {
    for {
      results <- Future.sequence(messageList.map(processMessage))
      resultResponse = messageList.zip(results).map {
        case (x, Right(_)) => ConsumeRequest.Result(ok = true, id = x.id)
        case (x, Left(e)) => ConsumeRequest.Result(ok = false, message = e.getMessage, id = x.id)
      }
      _ = sendRequest(ConsumeRequest(resultList = resultResponse))
    } yield ()
  }

  private def processMessage(message: ConsumeResponse.Message): Future[Either[Throwable, Unit]] = {
    val zonedDateTime: ZonedDateTime = if (message.timestamp.nonEmpty) {
      Try(ZonedDateTime.parse(message.timestamp)) match {
        case Success(value) =>
          value
        case Failure(e: Throwable) =>
          listener.logger(s"Can't parse timestamp '${message.timestamp}': ${e.getMessage}")
          ZonedDateTime.now
      }
    } else ZonedDateTime.now
    runWithTimeout(listener.consumeTimeout) {
      for {
        isProcessed <- listener.checkEventProcessedDatabase match {
          case Some (db) if message.id.nonEmpty =>
            isEventProcessed(db, redbus.consumer.Option.EventKey(topic, group, message.idempotencyKey, zonedDateTime))
          case _ =>
            Future.successful(false)
        }
        result <- if (isProcessed) {
          listener.logger(s"Skip already processed message $group / $topic / ${message.id}")
          Future.successful(Right(()))
        } else {
          val meta = MessageMeta(
            version = if (message.version == 0) None else Some(message.version),
            timestamp = if (message.timestamp.isEmpty) None else Some(ZonedDateTime.parse(message.timestamp)),
          )
          for {
            result <- processor(message.data.toByteArray, meta)
              .map(_ => Right(()))
              .recover(e => Left(e))
            _ <- (result, listener.checkEventProcessedDatabase) match {
              case (Right(_), Some(db)) if message.id.nonEmpty =>
                setEventProcessed(db, redbus.consumer.Option.EventKey(topic, group, message.id, zonedDateTime))
              case _ =>
                Future.unit
            }
          } yield result
        }
      } yield result
    }
  }

  private def runWithPause(duration: FiniteDuration)(fn: => Unit): Unit = {
    val pr = Promise[Unit]()
    actorSystem.scheduler.scheduleOnce(duration) {
      pr.complete(Try(()))
    }
    pr.future.map(_ => fn)
  }

  private def runWithTimeout[T](duration: FiniteDuration)(f: Future[T])
    (implicit ec: ExecutionContext, actorSystem: ActorSystem): Future[T] = {
    val timeoutFuture = after(duration, actorSystem.scheduler)(
      Future.failed(new TimeoutException("Future timed out"))
    )
    Future.firstCompletedOf(Seq(f, timeoutFuture))
  }

  private def isEventProcessed(db: Database, eventKey: redbus.consumer.Option.EventKey): Future[Boolean] = {
    db.run(IncomeMessages.isProcessed(eventKey.group, eventKey.topic, eventKey.idempotencyKey))
  }

  private def setEventProcessed(db: Database, eventKey: redbus.consumer.Option.EventKey): Future[_] = {
    db.run(IncomeMessages.setProcessed(eventKey.group, eventKey.topic, eventKey.idempotencyKey))
  }

}
