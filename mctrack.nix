{ lib
, buildGo118Module
, rev
}:

buildGo118Module {
  pname = "mctrack";
  version = rev;

  src = lib.cleanSource ./.;
  vendorSha256 = null;

  ldflags = [ "-s" "-w" ];
  subPackages = [ "cmd/mctrack" ];
}
