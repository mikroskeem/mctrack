{ lib
, buildGo118Module
, rev
}:

buildGo118Module {
  pname = "mctrack";
  version = rev;

  src = lib.cleanSource ./.;
  vendorSha256 = "sha256-D8Ii7askrmOLY1yUzbNCyIody4e9uaEhAiIUE7WMIjM=";

  ldflags = [ "-s" "-w" ];
  subPackages = [ "cmd/mctrack" ];
}
