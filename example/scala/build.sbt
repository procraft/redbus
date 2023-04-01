name := "redbus-example"
organization := "sergiusd"
version := "0.0.1"

scalaVersion in ThisBuild := "2.13.3"

//lazy val root = (project in file("."))
//  .dependsOn(redbusClient).aggregate(redbusClient)

//lazy val redbusClient = ProjectRef(file("../../api/scala"), "redbus")

libraryDependencies ++= Seq(
    "sergiusd" %% "redbus" % "0.0.5",
)

resolvers += "Artifactory" at "https://" + sys.env.getOrElse("MAVEN_HOST", "") + "/artifactory/sbt/"
credentials += Credentials(
    "Artifactory Realm",
    sys.env.getOrElse("MAVEN_HOST", ""),
    sys.env.getOrElse("MAVEN_USER", ""),
    sys.env.getOrElse("MAVEN_PASSWORD", "")
)