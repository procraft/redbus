name := "redbus"
organization := "sergiusd"
version := "0.0.5"

scalaVersion in ThisBuild := "2.13.12"
scalacOptions := Seq("-unchecked", "-deprecation", "-feature", "-encoding", "utf8")

PB.targets in Compile := Seq(
  scalapb.gen() -> (sourceManaged in Compile).value,
)

libraryDependencies ++= Seq(
  "io.grpc" % "grpc-netty" % scalapb.compiler.Version.grpcJavaVersion,
  "com.thesamet.scalapb" %% "scalapb-runtime-grpc" % scalapb.compiler.Version.scalapbVersion,
  "com.thesamet.scalapb" %% "scalapb-runtime" % scalapb.compiler.Version.scalapbVersion % "protobuf",
  "com.typesafe.akka" %% "akka-actor" % "2.6.8",
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

sources in doc := Seq.empty
publishArtifact in packageDoc := false
