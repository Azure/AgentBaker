define retrycmd_if_failure
	retries=$1; wait_sleep=$2; cmd=$3; \
	echo "Retries: $(retries)"; \
	echo "Wait Sleep: $(wait_sleep)"; \
	echo "Command: $(cmd)"; \
  	for i in $$(seq 1 $$retries); do \
    	$(cmd) && break || \
      	if [ $$i -eq $$retries ]; then \
        	echo Executed $$(basename $(cmd)) $$i times; \
        	exit 1; \
				else \
					sleep $(wait_sleep); \
				fi; \
		done; \
	echo Executed $$(basename $(cmd)) $$i times;
endef