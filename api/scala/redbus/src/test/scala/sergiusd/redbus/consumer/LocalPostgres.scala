package sergiusd.redbus.consumer

import java.sql.DriverManager

/**
 * Local PostgreSQL for specs that must exercise real SQL. Connects to the maintenance database
 * `postgres` by default; `REDBUS_PG_SPEC_URL` overrides the JDBC URL and
 * `REDBUS_PG_SPEC=false|0|off` switches the specs off. An unreachable server cancels the specs with
 * the reason instead of failing them. Specs create only uniquely named schemas and drop them.
 */
object LocalPostgres {
  val EnabledVariable = "REDBUS_PG_SPEC"
  val UrlVariable = "REDBUS_PG_SPEC_URL"
  val DefaultUrl = "jdbc:postgresql://localhost/postgres"

  def JdbcUrl: String = sys.env.getOrElse(UrlVariable, DefaultUrl)

  lazy val availability: Either[String, Unit] = sys.env.get(EnabledVariable).map(_.trim.toLowerCase) match {
    case Some("false") | Some("0") | Some("off") => Left(s"$EnabledVariable is off")
    case _ =>
      try {
        // sbt's test class loader does not auto-register JDBC drivers for DriverManager.
        Class.forName("org.postgresql.Driver")
        val connection = DriverManager.getConnection(JdbcUrl)
        try connection.createStatement().execute("SELECT 1") finally connection.close()
        Right(())
      } catch {
        case e: Throwable => Left(s"PostgreSQL is unavailable at $JdbcUrl: ${e.getMessage}")
      }
  }

  def uniqueName(prefix: String): String = s"${prefix}_${System.nanoTime() % 1000000000L}"
}
