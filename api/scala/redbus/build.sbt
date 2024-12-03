name := "redbus"
organization := "sergiusd"
version := "0.0.7"

ThisBuild / scalaVersion := "2.13.12"
scalacOptions := Seq("-unchecked", "-deprecation", "-feature", "-encoding", "utf8")

Compile / PB.targets := Seq(
  scalapb.gen() -> (Compile / sourceManaged).value,
)

libraryDependencies ++= Seq(
  "io.grpc" % "grpc-netty" % scalapb.compiler.Version.grpcJavaVersion,
  "com.thesamet.scalapb" %% "scalapb-runtime-grpc" % scalapb.compiler.Version.scalapbVersion,
  "com.thesamet.scalapb" %% "scalapb-runtime" % scalapb.compiler.Version.scalapbVersion % "protobuf",
  "com.typesafe.akka" %% "akka-actor" % "2.8.6",
  "com.typesafe.akka" %% "akka-stream" % "2.8.6"
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
