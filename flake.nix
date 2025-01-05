{
  inputs = {
    nixpkgs.url = "nixpkgs/nixos-unstable";
    flake-utils.url = "github:numtide/flake-utils";
  };

  outputs =
    {
      nixpkgs,
      flake-utils,
      ...
    }:
    flake-utils.lib.eachDefaultSystem (
      system:
      let
        pkgs = nixpkgs.legacyPackages.${system};
        lib = pkgs.lib;
      in
      {
        packages.default = pkgs.buildGoModule {
          name = "latex-build";
          src = lib.cleanSource ./.;
          vendorHash = "sha256-hE34LiA/4x1+ncoduykEDacVqVPMQQd4Re0m13kGbEs=";
        };

        devShells.default = pkgs.mkShell {
          packages = with pkgs; [
            gopls
            delve
            air
          ];
          nativeBuildInputs = with pkgs; [
            go
          ];
        };
      }
    );
}
