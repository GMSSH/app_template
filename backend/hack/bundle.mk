# Assemble one package directory with the GMSSH plugin layout.
#
# Usage:
#   make bundle DEST=dist/linux-amd64 GOOS=linux GOARCH=amd64
#   make bundle DEST=dist/app BIN=dist/backend/main.dist/main
#
# Layout (platform spec: app/bin/main must sit next to config.json and i18n/):
#   DEST/
#     install.sh  uninstall.sh
#     app/bin/main  app/bin/config.json  app/bin/i18n/
#     config/  data/config.json  logs/  tmp/

.PHONY: bundle linux-amd64 linux-arm64 build

build: linux-amd64 linux-arm64

linux-amd64:
	@$(MAKE) bundle DEST=$(DIST_DIR)/linux-amd64 GOOS=linux GOARCH=amd64 FORCE_BUILD=1

linux-arm64:
	@$(MAKE) bundle DEST=$(DIST_DIR)/linux-arm64 GOOS=linux GOARCH=arm64 FORCE_BUILD=1

bundle:
	@test -n "$(DEST)" || (echo "DEST is required (e.g. DEST=dist/linux-amd64)" >&2; exit 1)
	@mkdir -p "$(DEST)/app/bin" "$(DEST)/data" "$(DEST)/logs" "$(DEST)/tmp" "$(DEST)/config"
	@if [ -n "$(FORCE_BUILD)" ] || [ -n "$(GOOS)" ] || [ -n "$(GOARCH)" ]; then 		$(MAKE) build-one OUT="$(DEST)/app/bin/main" GOOS="$(GOOS)" GOARCH="$(GOARCH)"; 	elif [ -n "$(BIN)" ]; then 		test -f "$(BIN)" || (echo "BIN not found: $(BIN)" >&2; exit 1); 		cp -f "$(BIN)" "$(DEST)/app/bin/main"; 		chmod 755 "$(DEST)/app/bin/main"; 	elif [ ! -f "$(DEST)/app/bin/main" ]; then 		$(MAKE) build-one OUT="$(DEST)/app/bin/main"; 	fi
	@if [ -f configs/config.json ]; then 		cp -f configs/config.json "$(DEST)/data/config.json"; 		cp -f configs/config.json "$(DEST)/app/bin/config.json"; 	else 		echo "configs/config.json missing" >&2; 	fi
	@if [ -d configs ]; then 		mkdir -p "$(DEST)/config"; 		cp -f configs/*.json "$(DEST)/config/" 2>/dev/null || true; 	fi
	@if [ -d i18n ]; then 		rm -rf "$(DEST)/app/bin/i18n"; 		mkdir -p "$(DEST)/app/bin/i18n"; 		cp -f i18n/zh-CN.ini i18n/en.ini "$(DEST)/app/bin/i18n/" 2>/dev/null || true; 	fi
	@if [ -d script ]; then 		rm -rf "$(DEST)/script"; 		cp -R script "$(DEST)/script"; 	fi
	@if [ -f script/install.sh ]; then 		cp -f script/install.sh "$(DEST)/install.sh"; 		chmod +x "$(DEST)/install.sh"; 		echo "placed install hook: $(DEST)/install.sh"; 	fi
	@if [ -f script/uninstall.sh ]; then 		cp -f script/uninstall.sh "$(DEST)/uninstall.sh"; 		chmod +x "$(DEST)/uninstall.sh"; 		echo "placed uninstall hook: $(DEST)/uninstall.sh"; 	fi
	@echo "bundle ready: $(DEST)"
