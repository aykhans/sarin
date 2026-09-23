{
  lib,
  buildGoModule,
  go_1_27,
  rev ? "unknown",
  buildDate ? "unknown",
}:

(buildGoModule.override { go = go_1_27; }) (finalAttrs: {
  pname = "sarin";
  version = "1.4.4"; # bump per release

  src = lib.cleanSource ../.;

  vendorHash = "sha256-R5g79goG/VpSlH+hpvStUGIl+VWwmo7DTCO3j4rT0CU=";

  subPackages = [ "cmd/cli" ];

  env.CGO_ENABLED = 0; # fully static binary

  ldflags = [
    "-s"
    "-w"
    "-X=go.aykhans.me/sarin/internal/version.Version=v${finalAttrs.version}"
    "-X=go.aykhans.me/sarin/internal/version.GitCommit=${rev}"
    "-X=go.aykhans.me/sarin/internal/version.BuildDate=${buildDate}"
  ];

  preBuild = ''
    ldflags+=("-X 'go.aykhans.me/sarin/internal/version.GoVersion=$(go version)'")
  '';

  postInstall = ''
    mv $out/bin/cli $out/bin/sarin
  '';

  meta = {
    description = "High-performance HTTP load testing tool built with Go and fasthttp";
    homepage = "https://github.com/aykhans/sarin";
    license = lib.licenses.mit;
    mainProgram = "sarin";
    maintainers = [ ];
  };
})
