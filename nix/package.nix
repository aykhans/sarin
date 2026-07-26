{ lib
, buildGoModule
, go_1_26
, rev ? "unknown"
, buildDate ? "unknown"
}:

(buildGoModule.override { go = go_1_26; }) (finalAttrs: {
  pname = "sarin";
  version = "1.4.0"; # bump per release

  src = lib.cleanSource ../.;

  vendorHash = "sha256-/r2mioVoMbrboumF0sjHhharkGImQAShmiOQtdS5DaE=";

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

  # cmd/cli produces a binary named "cli"; rename it to "sarin".
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
