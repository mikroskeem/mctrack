{
  description = "mctrack";

  inputs = {
    nixpkgs.url = "github:NixOS/nixpkgs/nixpkgs-unstable";
    flake-utils.url = "github:numtide/flake-utils";
    docker-tools.url = "github:ZentriaMC/docker-tools/24aa170088c8d42446693de6d124e95bb890b3fe";
  };

  outputs = { self, nixpkgs, flake-utils, docker-tools }:
    let
      supportedSystems = [
        "aarch64-darwin"
        "aarch64-linux"
        "x86_64-darwin"
        "x86_64-linux"
      ];
    in
    flake-utils.lib.eachSystem supportedSystems (system:
      let
        rev = self.rev or "dirty";
        pkgs = nixpkgs.legacyPackages.${system};
      in
      rec {
        packages.mctrack = pkgs.callPackage ./mctrack.nix {
          inherit rev;
        };

        packages.mctrack-docker = pkgs.callPackage
          ({ lib, cacert, dockerTools, dumb-init, mctrack, name ? "mctrack", tag ? mctrack.version }: dockerTools.buildLayeredImage rec {
            inherit name tag;
            config = {
              Env = [
                "PATH=${lib.makeBinPath [ dumb-init mctrack ]}"
              ];
              Labels = {
                "org.opencontainers.image.source" = "https://github.com/mikroskeem/mctrack";
              };
              Entrypoint = [ "${dumb-init}/bin/dumb-init" "--" ];
              Cmd = [ "mctrack" ];
            };

            extraCommands = ''
              mkdir -p data
              ${docker-tools.lib.symlinkCACerts { inherit cacert; }}
            '';
          })
          {
            inherit (packages) mctrack;
          };

        devShell = pkgs.mkShell {
          nativeBuildInputs = [
            pkgs.go_1_18
            pkgs.golangci-lint
            pkgs.gopls
          ];
        };
      });
}
