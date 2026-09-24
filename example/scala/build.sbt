name := "redbus-example"
organization := "sergiusd"
version := "0.0.1"

ThisBuild / scalaVersion := "2.13.18"
ThisBuild / versionScheme := Some("semver-spec")
scalacOptions := Seq("-unchecked", "-deprecation", "-feature", "-encoding", "utf8")

lazy val root = (project in file("."))
  .dependsOn(redbusClient).aggregate(redbusClient)
lazy val redbusClient = ProjectRef(file("../../api/scala/redbus"), "redbus")

libraryDependencies ++= Seq(
    "io.github.cdimascio" % "dotenv-java" % "3.0.0",
)
