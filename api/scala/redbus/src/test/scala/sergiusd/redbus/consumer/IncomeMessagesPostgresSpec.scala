package sergiusd.redbus.consumer

import org.scalatest.BeforeAndAfterAll
import org.scalatest.matchers.should.Matchers
import org.scalatest.wordspec.AnyWordSpec
import slick.jdbc.PostgresProfile.api._

import java.sql.DriverManager
import scala.concurrent.duration._
import scala.concurrent.{Await, ExecutionContext, Future}

/**
 * `IncomeMessages` against a real PostgreSQL (see [[LocalPostgres]]). Each run creates two
 * throwaway schemas holding `redbus_inbox`: one exactly as `api/inbox.sql` defines it, and one whose
 * `created_at` has no default, as in client databases that renamed an older table.
 */
class IncomeMessagesPostgresSpec extends AnyWordSpec with Matchers with BeforeAndAfterAll {

  implicit private val ec: ExecutionContext = ExecutionContext.global

  private val base = LocalPostgres.uniqueName("redbus_inbox_spec")
  private val withDefault = s"${base}_default"
  private val withoutDefault = s"${base}_nodefault"

  private lazy val db = Database.forURL(LocalPostgres.JdbcUrl, driver = "org.postgresql.Driver")

  private def enabled = LocalPostgres.availability.isRight

  private def await[T](f: Future[T]): T = Await.result(f, 10.seconds)

  private def run[T](action: DBIO[T]): T = await(db.run(action))

  private def createInbox(schema: String, createdAtDefault: String): DBIO[Int] =
    sqlu"""CREATE SCHEMA #$schema""" andThen
      sqlu"""CREATE TABLE #$schema.redbus_inbox (
               "key" VARCHAR NOT NULL PRIMARY KEY,
               created_at TIMESTAMPTZ NOT NULL #$createdAtDefault
             )"""

  override def beforeAll(): Unit = {
    LocalPostgres.availability.left.foreach(reason => println(s"IncomeMessagesPostgresSpec cancelled: $reason"))
    if (enabled) {
      run(createInbox(withDefault, "DEFAULT NOW()"))
      run(createInbox(withoutDefault, ""))
    }
  }

  override def afterAll(): Unit = if (enabled) {
    try run(
      sqlu"""DROP SCHEMA IF EXISTS #$withDefault CASCADE""" andThen
        sqlu"""DROP SCHEMA IF EXISTS #$withoutDefault CASCADE"""
    ) finally db.close()
  }

  private def requirePostgres(): Unit =
    assume(enabled, LocalPostgres.availability.left.getOrElse(""))

  Seq(
    "a table as in api/inbox.sql" -> withDefault,
    "a table whose created_at has no default" -> withoutDefault,
  ).foreach { case (shape, schema) =>
    s"IncomeMessages.claim on $shape" should {
      "insert the mark once and report false for a repeated claim" in {
        requirePostgres()
        run(IncomeMessages.claimIn(schema, "group", "topic", "claim-twice")) shouldBe true
        run(IncomeMessages.claimIn(schema, "group", "topic", "claim-twice")) shouldBe false
        run(IncomeMessages.isProcessedIn(schema, "group", "topic", "claim-twice")) shouldBe true
      }

      "share the key with setProcessed in both directions" in {
        requirePostgres()
        run(IncomeMessages.setProcessedIn(schema, "group", "topic", "marked-before"))
        run(IncomeMessages.claimIn(schema, "group", "topic", "marked-before")) shouldBe false

        run(IncomeMessages.claimIn(schema, "group", "topic", "claimed-before")) shouldBe true
        run(IncomeMessages.isProcessedIn(schema, "group", "topic", "claimed-before")) shouldBe true
      }

      "keep group, topic and key apart" in {
        requirePostgres()
        run(IncomeMessages.claimIn(schema, "group", "topic", "k")) shouldBe true
        run(IncomeMessages.claimIn(schema, "other", "topic", "k")) shouldBe true
        run(IncomeMessages.claimIn(schema, "group", "other", "k")) shouldBe true
      }

      "release the key when the caller's transaction rolls back" in {
        requirePostgres()
        val failing = IncomeMessages.claimIn(schema, "group", "topic", "rolled-back")
          .flatMap(_ => DBIO.failed(new IllegalStateException("business write failed")))
          .transactionally
        an[IllegalStateException] should be thrownBy run(failing)

        run(IncomeMessages.isProcessedIn(schema, "group", "topic", "rolled-back")) shouldBe false
        run(IncomeMessages.claimIn(schema, "group", "topic", "rolled-back").transactionally) shouldBe true
      }
    }
  }

  "Inbox.guard" should {
    def business(key: String): DBIO[Int] =
      sql"""SELECT count(*) FROM #$withoutDefault.business WHERE "key" = $key""".as[Int].head

    def write(key: String): DBIO[Unit] =
      sqlu"""INSERT INTO #$withoutDefault.business ("key") VALUES ($key)""".map(_ => ())

    def guarded(key: String, claim: Option[Model.InboxClaim]): (DBIO[Unit], () => Int) = {
      var skips = 0
      (Inbox.guard(claim, skips += 1)(write(key)).transactionally, () => skips)
    }

    "run the business writes on a successful claim and skip them on a repeated one" in {
      requirePostgres()
      run(sqlu"""CREATE TABLE IF NOT EXISTS #$withoutDefault.business ("key" VARCHAR NOT NULL)""")
      val claim = IncomeMessages.claimIn(withoutDefault, "group", "topic", "guarded")

      val (first, firstSkips) = guarded("guarded", Some(claim))
      run(first)
      firstSkips() shouldBe 0

      val (second, secondSkips) = guarded("guarded", Some(claim))
      run(second)
      secondSkips() shouldBe 1

      run(business("guarded")) shouldBe 1
    }

    "always run the business writes without a claim" in {
      requirePostgres()
      run(sqlu"""CREATE TABLE IF NOT EXISTS #$withoutDefault.business ("key" VARCHAR NOT NULL)""")
      run(guarded("unclaimed", None)._1)
      run(guarded("unclaimed", None)._1)

      run(business("unclaimed")) shouldBe 2
    }
  }

  "A concurrent claim of the same key" should {
    // The first claim runs in a raw JDBC session so its transaction stays open while the second waits.
    def withOpenClaim[A](schema: String, key: String)(f: java.sql.Connection => A): A = {
      val holder = DriverManager.getConnection(LocalPostgres.JdbcUrl)
      holder.setAutoCommit(false)
      try {
        val statement = holder.prepareStatement(
          s"""INSERT INTO $schema.redbus_inbox ("key", created_at) VALUES (?, now()) ON CONFLICT ("key") DO NOTHING"""
        )
        statement.setString(1, s"group|topic|$key")
        statement.executeUpdate() shouldBe 1
        f(holder)
      } finally {
        holder.rollback()
        holder.close()
      }
    }

    def waits(f: Future[Boolean]): Boolean =
      try { Await.result(f, 500.millis); false } catch { case _: java.util.concurrent.TimeoutException => true }

    "wait for the first transaction and get false after its commit" in {
      requirePostgres()
      withOpenClaim(withoutDefault, "concurrent-commit") { holder =>
        val second = db.run(IncomeMessages.claimIn(withoutDefault, "group", "topic", "concurrent-commit").transactionally)
        waits(second) shouldBe true
        holder.commit()
        await(second) shouldBe false
      }
    }

    "wait for the first transaction and claim the key after its rollback" in {
      requirePostgres()
      withOpenClaim(withoutDefault, "concurrent-rollback") { holder =>
        val second = db.run(IncomeMessages.claimIn(withoutDefault, "group", "topic", "concurrent-rollback").transactionally)
        waits(second) shouldBe true
        holder.rollback()
        await(second) shouldBe true
      }
    }
  }
}
