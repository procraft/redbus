name := "redbus-example"
organization := "sergiusd"
version := "0.0.1"

ThisBuild / scalaVersion := "2.13.12"
ThisBuild / versionScheme := Some("semver-spec")
scalacOptions := Seq("-unchecked", "-deprecation", "-feature", "-encoding", "utf8")

lazy val root = (project in file("."))
  //.dependsOn(redbusClient).aggregate(redbusClient)
//lazy val redbusClient = ProjectRef(file("../../api/scala/redbus"), "redbus")

libraryDependencies ++= Seq(
    "sergiusd" %% "redbus" % "0.0.15",
)

resolvers += "Artifactory" at "https://maven.libicraft.ru/artifactory/sbt/"
credentials += Credentials(
    "Artifactory Realm",
    sys.env.getOrElse("MAVEN_HOST", ""),
    sys.env.getOrElse("MAVEN_USER", ""),
    sys.env.getOrElse("MAVEN_PASSWORD", "")
)