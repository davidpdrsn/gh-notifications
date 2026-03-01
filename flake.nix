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

        trolleyAsset = {
          x86_64-linux = {
            cliFile = "trolley-cli-x86_64-linux.tar.xz";
            cliHash = "sha256-+BY6aoGo20QybIh9kkJNYwgor/AazGvTRP4V6+HrLe8=";
            runtimeFile = "trolley-runtime-x86_64-linux.tar.xz";
            runtimeHash = "sha256-2lTovpjQFvUu04TOFxdHtfOmrgJ6uZ1pj1TvFbO0ZNk=";
          };
          aarch64-linux = {
            cliFile = "trolley-cli-aarch64-linux.tar.xz";
            cliHash = "sha256-aONnuxDqsOva+ah4M56TO3iZhEiFKpqZAPtjmlDtDf0=";
            runtimeFile = "trolley-runtime-aarch64-linux.tar.xz";
            runtimeHash = "sha256-yDaqPE36pXyrYVDd8A0S6DvndkdJNtz9vA/e4ajSIgo=";
          };
          x86_64-darwin = {
            cliFile = "trolley-cli-x86_64-macos.tar.xz";
            cliHash = "sha256-mKYuqLi4UEFxJMTRCFoD48220TFl+wm7+yL2r/LU1lQ=";
            runtimeFile = "trolley-runtime-x86_64-macos.tar.xz";
            runtimeHash = "sha256-YHTrGpu1YrPjocMteor0cfvES6ktu9bDAvY/WdpFfcw=";
          };
          aarch64-darwin = {
            cliFile = "trolley-cli-aarch64-macos.tar.xz";
            cliHash = "sha256-nW3HYY0W8026KCUCXU3+z2zgxJgGEjmlR586jlYSGJQ=";
            runtimeFile = "trolley-runtime-aarch64-macos.tar.xz";
            runtimeHash = "sha256-Z++RwrJOsts9ejIAfIHPzNC75IAuWt1hqJY4fYXvGUs=";
          };
        }.${system};

        trolleyCli = pkgs.stdenvNoCC.mkDerivation {
          pname = "trolley";
          version = "0.5.0";

          nativeBuildInputs = pkgs.lib.optionals pkgs.stdenv.isDarwin [ pkgs.darwin.cctools ];

          cliSrc = pkgs.fetchurl {
            url = "https://github.com/weedonandscott/trolley/releases/download/v0.5.0/${trolleyAsset.cliFile}";
            hash = trolleyAsset.cliHash;
          };

          runtimeSrc = pkgs.fetchurl {
            url = "https://github.com/weedonandscott/trolley/releases/download/v0.5.0/${trolleyAsset.runtimeFile}";
            hash = trolleyAsset.runtimeHash;
          };

          dontUnpack = true;

          installPhase = ''
            runHook preInstall

            mkdir -p "$out/bin" "$out/libexec"

            tar -xJf "$cliSrc"
            install -m755 trolley "$out/libexec/trolley-cli"
            rm trolley

            tar -xJf "$runtimeSrc"
            install -m755 trolley "$out/libexec/trolley-runtime"

            ${pkgs.lib.optionalString pkgs.stdenv.isDarwin ''
              install_name_tool \
                -change /opt/homebrew/opt/xz/lib/liblzma.5.dylib ${pkgs.xz.out}/lib/liblzma.5.dylib \
                "$out/libexec/trolley-cli"
            ''}

            cat > "$out/bin/trolley" <<EOF
            #!${pkgs.runtimeShell}
            export TROLLEY_RUNTIME_SOURCE="$out/libexec/trolley-runtime"
            exec "$out/libexec/trolley-cli" "\$@"
            EOF
            chmod +x "$out/bin/trolley"

            runHook postInstall
          '';
        };

        gh-pr = pkgs.buildGo124Module {
          pname = "gh-pr";
          version = "0.1.0";
          src = self;
          subPackages = [ "cmd/gh-pr" ];
          vendorHash = "sha256-Kjdyv//1yoa0Xi3tflu6BRG77lANI1ssL2ZdTaTn2u4=";
        };

        app = if pkgs.stdenv.isDarwin then pkgs.stdenvNoCC.mkDerivation {
          pname = "gh-pr-app";
          version = "0.1.0";
          src = self;
          nativeBuildInputs = [ pkgs.go trolleyCli ];

          dontConfigure = true;
          dontFixup = true;

          buildPhase = ''
            runHook preBuild

            cp -R "$src" source
            chmod -R +w source
            cd source

            export HOME="$TMPDIR"
            export CGO_ENABLED=0

            GOOS=darwin GOARCH=arm64 go build -o bin/gh-pr-tui-darwin-arm64 ./cmd/gh-pr-tui
            GOOS=darwin GOARCH=amd64 go build -o bin/gh-pr-tui-darwin-amd64 ./cmd/gh-pr-tui

            trolley package --formats mac-app

            runHook postBuild
          '';

          installPhase = ''
            runHook preInstall

            mkdir -p "$out"
            app_path="$(find trolley/build -type d -name '*.app' | head -n1)"
            if [ -z "$app_path" ]; then
              echo "No .app bundle produced by trolley" >&2
              exit 1
            fi

            cp -R "$app_path" "$out/gh-pr.app"

            substituteInPlace "$out/gh-pr.app/Contents/Resources/ghostty.conf" \
              --replace-fail 'command = direct:./gh-notifications_core' "command = direct:$out/gh-pr.app/Contents/MacOS/gh-notifications_core"

            runHook postInstall
          '';
        } else pkgs.runCommand "gh-pr-app-unsupported" { } ''
          echo "gh-pr app bundling is only supported on Darwin" >&2
          exit 1
        '';
      in {
        packages.gh-pr = gh-pr;
        packages.app = app;
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
          ] ++ [
            trolleyCli
          ];

          shellHook = ''
            echo "gh-pr dev shell ready"
            echo "Go: $(go version)"
          '';
        };
      });
}
