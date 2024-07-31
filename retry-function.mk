define retrycmd
    @success=0; \
    cmd=$(1); \
    retries=$(2); \
		target=$$(basename $$(echo $(cmd))); \
    for i in $$(seq 1 $(retries)); do \
        $(cmd) && { success=1; break; } || echo "$(target) failed. Retrying..."; \
        sleep 3; \
    done; \
    if [ $$success -ne 1 ]; then \
        echo "$(target) failed after $(retries) attempts."; \
				exit 1; \
		else
			echo "$(target) succeeded after $(retries) attempts."; \
    fi
endef