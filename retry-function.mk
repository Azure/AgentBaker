define retrycmd
    @success=0; \
    cmd=$(1); \
    retries=$(2); \
		target=$$(basename $$(echo $$(cmd))); \
		last_attempt=0; \
    for i in $$(seq 1 $$(retries)); do \
        $$(cmd) && { success=1; last_attempt=$$i; break; } || echo "$$target failed. Retrying..."; \
        sleep 3; \
    done; \
    if [ $$success -ne 1 ]; then \
        echo "$$target failed after $$last_attempt attempts."; \
				exit 1; \
		else \
			echo "$$target succeeded after $$last_attempt attempts."; \
    fi
endef