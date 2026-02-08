{
  description = "Development shell for gh-pr";

  inputs = {
    nixpkgs.url = "github:NixOS/nixpkgs/nixos-unstable";
    flake-utils.url = "github:numtide/flake-utils";
  };

  outputs = { self, nixpkgs, flake-utils }:
    flake-utils.lib.eachDefaultSystem (system:
      let
        pkgs = import nixpkgs {
          inherit system;
        };

        gh-pr = pkgs.buildGoModule {
          pname = "gh-pr";
          version = "0.1.0";
          src = ./.;
          subPackages = [ "cmd/gh-pr" ];
          vendorHash = "sha256-sZUEzBxbButVYi8eFxyrqCQI51a8rUDXpvO1JUxSmjU=";
        };
      in {
        packages.gh-pr = gh-pr;
        packages.default = gh-pr;

        apps.default = {
          type = "app";
          program = "${gh-pr}/bin/gh-pr";
        };

        devShells.default = pkgs.mkShell {
          packages = with pkgs; [
            go
            gopls
            gotools
            delve
            golangci-lint
            oapi-codegen
            gh
            jq
            curl
            git
          ];

          shellHook = ''
            echo "gh-pr dev shell ready"
            echo "Go: $(go version)"
          '';
        };
      });
}
