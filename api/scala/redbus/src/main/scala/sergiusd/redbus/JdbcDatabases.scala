package sergiusd.redbus

import slick.jdbc.{JdbcBackend, PostgresProfile}

private[redbus] object JdbcDatabases {

  /**
   * Slick types a database by the path of its profile (`profile.backend.Database`), so the database
   * of an application's own `PostgresProfile` subclass is a different static type from
   * `PostgresProfile.backend.Database` although it is the same class: every `JdbcProfile` shares the
   * `JdbcBackend` object, and all `JdbcDatabaseDef` values erase to one class. Public SDK entry points
   * accept the profile-independent projection and narrow it here, so callers need no cast.
   */
  def postgres(db: JdbcBackend#JdbcDatabaseDef): PostgresProfile.backend.Database =
    db.asInstanceOf[PostgresProfile.backend.Database]
}
