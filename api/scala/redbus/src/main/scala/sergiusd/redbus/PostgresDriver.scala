package sergiusd.redbus

import com.github.tminglei.slickpg.PgPlayJsonSupport
import play.api.libs.json._
import sergiusd.redbus.producer.PublishingMessage
import slick.jdbc.{JdbcType, PostgresProfile}

trait PostgresDriver extends PostgresProfile with PgPlayJsonSupport {
  def pgjson = "jsonb"

  override val api: CustomAPI = new CustomAPI {}

  trait CustomAPI extends JdbcAPI with JsonImplicits {
    implicit val optionsColumnType: JdbcType[PublishingMessage.Options] = MappedJdbcType.base[PublishingMessage.Options, JsValue](
      Json.toJson(_),
      _.as[PublishingMessage.Options],
    )
  }
}

object PostgresDriver extends PostgresDriver
