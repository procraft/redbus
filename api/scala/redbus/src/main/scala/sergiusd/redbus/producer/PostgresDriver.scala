package sergiusd.redbus.producer

import com.github.tminglei.slickpg.PgPlayJsonSupport
import slick.jdbc.{JdbcType, PostgresProfile}
import play.api.libs.json._

trait PostgresDriver extends PostgresProfile with PgPlayJsonSupport {
  def pgjson = "jsonb"

  override val api: CustomAPI = new CustomAPI {}

  trait CustomAPI extends JdbcAPI with JsonImplicits {
    implicit val optionsColumnType: JdbcType[PublishingMessage.Options] =
      MappedJdbcType.base[PublishingMessage.Options, JsValue](Json.toJson(_), _.as[PublishingMessage.Options])

//    implicit val zonedDateTimeType: JdbcType[ZonedDateTime] with BaseTypedType[ZonedDateTime] = {
//      MappedColumnType.base[ZonedDateTime, java.sql.Timestamp](
//        zdt => java.sql.Timestamp.from(zdt.toInstant),
//        t => ZonedDateTime.ofInstant(t.toInstant, ZoneId.systemDefault())
//      )
//    }
  }
}

object PostgresDriver extends PostgresDriver
