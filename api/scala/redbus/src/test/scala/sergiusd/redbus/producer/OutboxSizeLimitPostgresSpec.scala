package sergiusd.redbus.producer

import org.scalatest.BeforeAndAfterAll
import org.scalatest.matchers.should.Matchers
import org.scalatest.wordspec.AnyWordSpec
import sergiusd.redbus.consumer.LocalPostgres
import slick.jdbc.PostgresProfile.api._

import scala.concurrent.duration._
import scala.concurrent.{Await, ExecutionContext}
import scala.util.{Failure, Try}

/**
 * The outbox size limit against a real PostgreSQL (see [[LocalPostgres]]). `produceDba` always
 * targets `public.redbus_outbox`, so every test creates that table inside its own transaction and
 * ends it with a rollback: nothing outlives the test. A database that already has the table cancels
 * the tests instead of touching it.
 */
class OutboxSizeLimitPostgresSpec extends AnyWordSpec with Matchers with BeforeAndAfterAll {

  implicit private val ec: ExecutionContext = ExecutionContext.global

  private lazy val db = Database.forURL(LocalPostgres.JdbcUrl, driver = "org.postgresql.Driver")

  private def enabled = LocalPostgres.availability.isRight

  override def afterAll(): Unit = if (enabled) db.close()

  private object Discard extends RuntimeException("discard the test transaction")

  private val tableExists = sql"SELECT to_regclass('public.redbus_outbox') IS NOT NULL".as[Boolean].head

  private val createOutbox = sqlu"""CREATE TABLE public.redbus_outbox (
      id BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
      topic VARCHAR NOT NULL,
      message BYTEA NOT NULL,
      options JSONB NOT NULL,
      created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
    )"""

  private val rowSizes = sql"SELECT octet_length(message) FROM public.redbus_outbox ORDER BY id".as[Int]

  private def requireCleanPostgres(): Unit = {
    assume(enabled, LocalPostgres.availability.left.getOrElse(""))
    assume(!Await.result(db.run(tableExists), 10.seconds), "public.redbus_outbox already exists in the spec database")
  }

  private def payload(size: Int): Array[Byte] = Array.fill(size)(1.toByte)

  "Producer.produceDbaWithLimit on PostgreSQL" should {

    "write a payload of exactly the limit and no row for one byte more" in {
      requireCleanPostgres()
      @volatile var observed: (Try[Int], Vector[Int]) = null
      val action = for {
        _ <- createOutbox
        _ <- Producer.produceDbaWithLimit("topic", payload(10), 10)
        rejected <- Producer.produceDbaWithLimit("topic", payload(11), 10).asTry
        sizes <- rowSizes
        _ = observed = (rejected, sizes)
        _ <- DBIO.failed(Discard)
      } yield ()

      Try(Await.result(db.run(action.transactionally), 10.seconds)) shouldBe Failure(Discard)
      val (rejected, sizes) = observed
      rejected shouldBe Failure(MessageTooLargeException("topic", 11, 10))
      sizes shouldBe Vector(10)
    }

    "roll back the caller's transaction" in {
      requireCleanPostgres()
      val action = for {
        _ <- createOutbox
        _ <- Producer.produceDba("topic", payload(1))
        _ <- Producer.produceDba("topic", payload(Producer.defaultMaxMessageBytes + 1))
      } yield ()

      val e = the[MessageTooLargeException] thrownBy Await.result(db.run(action.transactionally), 10.seconds)
      e.sizeBytes shouldBe Producer.defaultMaxMessageBytes + 1
      // The table was created in the same transaction: its absence proves the whole rollback.
      Await.result(db.run(tableExists), 10.seconds) shouldBe false
    }
  }
}
