name := "redbus"
organization := "sergiusd"
version := "0.4.1"

ThisBuild / scalaVersion := "2.13.18"
ThisBuild / crossScalaVersions := Seq("2.13.18", "3.3.8")
ThisBuild / versionScheme := Some("semver-spec")
scalacOptions := Seq("-unchecked", "-deprecation", "-feature", "-encoding", "utf8")

Compile / PB.targets := Seq(
  scalapb.gen() -> (Compile / sourceManaged).value,
)

val pekkoVersion = "1.0.3"
val slickPgVersion = "0.23.1"
val slickHikaricp = "3.6.1"

libraryDependencies ++= Seq(
  "io.grpc" % "grpc-netty-shaded" % scalapb.compiler.Version.grpcJavaVersion,
  "com.thesamet.scalapb" %% "scalapb-runtime-grpc" % scalapb.compiler.Version.scalapbVersion,
  "com.thesamet.scalapb" %% "scalapb-runtime" % scalapb.compiler.Version.scalapbVersion % "protobuf",
  "org.apache.pekko" %% "pekko-actor" % pekkoVersion,
  "com.github.tminglei" %% "slick-pg" % slickPgVersion,
  "com.github.tminglei" %% "slick-pg_play-json" % slickPgVersion,
  "com.typesafe.slick" %% "slick-hikaricp" % slickHikaricp,
  "org.apache.pekko" %% "pekko-testkit" % pekkoVersion % Test,
  "org.scalatest" %% "scalatest" % "3.2.19" % Test,
)

val mavenHost = "maven.libicraft.ru"
val mavenUser = sys.env.get("MAVEN_USER").filter(_.nonEmpty)
val mavenPassword = sys.env.get("MAVEN_PASSWORD").filter(_.nonEmpty)
val mavenCredentialsFile = Path.userHome / ".sbt" / "1.0" / "credentials"

publishTo := Some(
  "Artifactory Realm" at s"https://$mavenHost/artifactory/sbt;build.timestamp=${new java.util.Date().getTime}"
)

credentials ++= ((mavenUser, mavenPassword) match {
  case (Some(user), Some(password)) =>
    Seq(Credentials("Artifactory Realm", mavenHost, user, password))
  case (None, None) if mavenCredentialsFile.isFile =>
    Seq(Credentials(mavenCredentialsFile))
  case _ =>
    Seq.empty
})

val validatePublishCredentials = taskKey[Unit]("Validate credentials required to publish the SDK")
validatePublishCredentials := {
  (mavenUser, mavenPassword) match {
    case (Some(_), Some(_)) => ()
    case (None, None) if mavenCredentialsFile.isFile => ()
    case (None, None) =>
      sys.error(
        s"Publishing requires both MAVEN_USER and MAVEN_PASSWORD, or a credentials file at $mavenCredentialsFile"
      )
    case _ =>
      sys.error("MAVEN_USER and MAVEN_PASSWORD must be set together when publishing")
  }
}

publish := publish.dependsOn(validatePublishCredentials).value

doc / sources := Seq.empty
packageDoc / publishArtifact := false
