{ lib
, stdenv
, buildGoModule
, go_1_26
, rev ? "unknown"
, buildDate ? "unknown"
}:

(buildGoModule.override { go = go_1_26; }) (finalAttrs: {
  pname = "sarin";
  version = "1.3.2"; # bump per release

  src = lib.cleanSource ../.;

  vendorHash = "sha256-BMH0alJeZoK/8cE9Y+s++G7D+Dg5xcZNTU4zkRMBpfU=";

  subPackages = [ "cmd/cli" ];

  env.CGO_ENABLED = 0; # fully static binary

  ldflags = [
    "-s"
    "-w"
    "-X=go.aykhans.me/sarin/internal/version.Version=v${finalAttrs.version}"
    "-X=go.aykhans.me/sarin/internal/version.GitCommit=${rev}"
    "-X=go.aykhans.me/sarin/internal/version.BuildDate=${buildDate}"
    # Value has spaces, so it is single-quoted: `go build` splits -ldflags
    # shell-style and honours the quotes, keeping this as one -X value.
    "-X 'go.aykhans.me/sarin/internal/version.GoVersion=go version go${go_1_26.version} ${stdenv.hostPlatform.go.GOOS}/${stdenv.hostPlatform.go.GOARCH}'"
  ];

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
