{ lib
, buildGo118Module
, rev
}:

buildGo118Module {
  pname = "mctrack";
  version = rev;

  src = lib.cleanSource ./.;
  vendorSha256 = "sha256-bMmlnp0iDPe2QEZgU8xQ21P6z6PqpYIaK73gjuvHa7E=";

  ldflags = [ "-s" "-w" ];
  subPackages = [ "cmd/mctrack" ];
}
