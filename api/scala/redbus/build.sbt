name := "redbus"
organization := "sergiusd"
version := "0.1.4"

ThisBuild / scalaVersion := "2.13.16"
scalacOptions := Seq("-unchecked", "-deprecation", "-feature", "-encoding", "utf8")

Compile / PB.targets := Seq(
  scalapb.gen() -> (Compile / sourceManaged).value,
)

val akkaVersion = "2.6.21"
val slickPgVersion = "0.22.2"

libraryDependencies ++= Seq(
  "io.grpc" % "grpc-netty" % scalapb.compiler.Version.grpcJavaVersion,
  "com.thesamet.scalapb" %% "scalapb-runtime-grpc" % scalapb.compiler.Version.scalapbVersion,
  "com.thesamet.scalapb" %% "scalapb-runtime" % scalapb.compiler.Version.scalapbVersion % "protobuf",
  "com.typesafe.akka" %% "akka-actor" % akkaVersion,
  "com.typesafe.akka" %% "akka-stream" % akkaVersion,
  "com.github.tminglei" %% "slick-pg" % slickPgVersion,
  "com.github.tminglei" %% "slick-pg_play-json" % slickPgVersion,
  "com.typesafe.slick" %% "slick-hikaricp" % "3.5.2",
)

publishTo := Some(
  "Artifactory Realm" at "https://" + sys.env.getOrElse("MAVEN_HOST", "") + "/artifactory/sbt;build.timestamp=" + new java.util.Date().getTime
)
credentials += Credentials(
    "Artifactory Realm",
    sys.env.getOrElse("MAVEN_HOST", ""),
    sys.env.getOrElse("MAVEN_USER", ""),
    sys.env.getOrElse("MAVEN_PASSWORD", "")
)

doc / sources := Seq.empty
packageDoc / publishArtifact := false
