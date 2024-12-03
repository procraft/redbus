package sergiusd.redbus.consumer

import akka.Done
import akka.actor.ActorSystem
import akka.pattern.after
import io.grpc.stub.StreamObserver
import sergiusd.redbus.api._

import java.util.concurrent.TimeUnit
import scala.concurrent.{ExecutionContext, Future, Promise, TimeoutException}
import scala.concurrent.duration.FiniteDuration
import scala.util.{Failure, Success, Try}

class Consumer(
  grpcClient: RedbusServiceGrpc.RedbusServiceStub,
  hostPort: String,
  topic: String,
  group: String,
  processor: Model.Processor,
  addStopHook: Model.StopHook,
  options: Option.ListenerOptionFn*,
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

    connectAndServe()

    addStopHook { () =>
      Future {
        isRunning = false
        promise.trySuccess(Done)
        println("Consumer shutting down")
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
        println("Stream completed")
        promise.success(Done)
      }
    }

    requestObserver = Some(grpcClient.consume(responseObserver))

    sendRequest(connectRequest)
  }

  private def reconnect(error: String): Unit = {
    requestObserver = None
    attempt += 1
    println(s"Connect to $hostPort error: $error, attempt $attempt, ${listener.unavailableTimeout} waiting...")
    runWithPause(listener.unavailableTimeout)(connectAndServe())
  }

  private def sendRequest(request: ConsumeRequest): Unit = {
    try {
      requestObserver.foreach(_.onNext(request))
    } catch {
      case e: Throwable => reconnect(e.getMessage)
    }
  }

  private def precessConnect(response: ConsumeResponse.Connect): Unit = {
    if (response.ok) {
      attempt = 0
      isConnected = true
      println(s"Connect to $hostPort, id = ${connectRequest.connect.map(_.id).getOrElse("?")}")
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
    runWithTimeout(listener.consumeTimeout) {
      processor(message.data.toByteArray)
        .map(_ => Right(()))
        .recover(e => Left(e))
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

}