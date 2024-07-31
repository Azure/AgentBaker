define retrycmd_if_failure
	retries=$1; wait_sleep=$2; timeout=$3; shift && shift && shift; \
  	for i in $$(seq 1 $$retries); do \
    	timeout $$timeout "$$@" && break || \
      	if [ $$i -eq $$retries ]; then \
        	echo Executed "$$@" $$i times; \
        	return 1; \
				else \
					sleep $$wait_sleep; \
				fi; \
		done; \
	echo Executed "$$@" $$i times;
endef