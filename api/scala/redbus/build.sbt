name := "redbus"
organization := "sergiusd"
version := "0.3.0"

ThisBuild / scalaVersion := "2.13.18"
ThisBuild / versionScheme := Some("semver-spec")
scalacOptions := Seq("-unchecked", "-deprecation", "-feature", "-encoding", "utf8")

Compile / PB.targets := Seq(
  scalapb.gen() -> (Compile / sourceManaged).value,
)

val pekkoVersion = "1.0.3"
val slickPgVersion = "0.23.1"
val slickHikaricp = "3.6.1"

libraryDependencies ++= Seq(
  "io.grpc" % "grpc-netty" % scalapb.compiler.Version.grpcJavaVersion,
  "com.thesamet.scalapb" %% "scalapb-runtime-grpc" % scalapb.compiler.Version.scalapbVersion,
  "com.thesamet.scalapb" %% "scalapb-runtime" % scalapb.compiler.Version.scalapbVersion % "protobuf",
  "org.apache.pekko" %% "pekko-actor" % pekkoVersion,
  "com.github.tminglei" %% "slick-pg" % slickPgVersion,
  "com.github.tminglei" %% "slick-pg_play-json" % slickPgVersion,
  "com.typesafe.slick" %% "slick-hikaricp" % slickHikaricp,
  "org.apache.pekko" %% "pekko-testkit" % pekkoVersion % Test,
  "org.scalatest" %% "scalatest" % "3.2.19" % Test,
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
