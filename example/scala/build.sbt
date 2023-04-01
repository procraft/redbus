name := "redbus-example"
organization := "sergiusd"
version := "0.0.1"

//lazy val root = (project in file("."))
//  .dependsOn(redbusClient).aggregate(redbusClient)

//lazy val redbusClient = ProjectRef(file("../../api/scala"), "redbus")

libraryDependencies ++= Seq(
    "sergiusd" % "redbus" % "0.0.1",
)
