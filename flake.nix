{
  description = "Sarin - high-performance HTTP load testing tool built with Go and fasthttp";

  inputs = {
    nixpkgs.url = "github:NixOS/nixpkgs/nixos-unstable";
  };

  outputs = { self, nixpkgs }:
    let
      inherit (nixpkgs) lib;

      systems = [
        "x86_64-linux"
        "aarch64-linux"
        "x86_64-darwin"
        "aarch64-darwin"
      ];

      forAllSystems = f: lib.genAttrs systems (system: f nixpkgs.legacyPackages.${system});

      rev = self.rev or self.dirtyRev or "unknown";

      # self.lastModifiedDate is "YYYYMMDDHHMMSS"; reshape to RFC 3339 (UTC).
      raw = self.lastModifiedDate or "19700101000000";
      s = i: n: lib.substring i n raw;
      buildDate = "${s 0 4}-${s 4 2}-${s 6 2}T${s 8 2}:${s 10 2}:${s 12 2}Z";
    in
    {
      packages = forAllSystems (pkgs: {
        default = pkgs.callPackage ./nix/package.nix {
          inherit rev buildDate;
        };
      });

      # nix run github:aykhans/sarin -- <args>
      apps = forAllSystems (pkgs: {
        default = {
          type = "app";
          program = "${self.packages.${pkgs.system}.default}/bin/sarin";
          meta.description = "High-performance HTTP load testing tool";
        };
      });

      devShells = forAllSystems (pkgs: {
        default = pkgs.mkShell {
          packages = [ pkgs.go_1_26 pkgs.golangci-lint pkgs.go-task ];
        };
      });

      # For downstream flakes: inputs.sarin.overlays.default
      overlays.default = final: _prev: {
        sarin = final.callPackage ./nix/package.nix { };
      };
    };
}
