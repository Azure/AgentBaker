include ./ev2ab.mk

BUILDING_FLAVOUR ?= official

OUTPUT_DIR ?= out
export BUILD_BUILDNUMBER ?= latest

$(EV2AB_SOURCE)/toggle.service: toggle.service
	mkdir -p $(@D)
	cp -a $< $(@D)

ifeq ($(BUILDING_PIPELINE),)
	cd .. && ./ci/build.sh
else
	cd .. && ./ci/build-in-makefile.sh
endif
	cp ../toggles.yaml $@/package/


#This if is for cdpx which does not support docker calls
#see https://dev.azure.com/msazure/CloudNativeCompute/_git/aks-config/commit/daf727934c4466fd1f8eaf72570da8719568c98b?refName=refs%2Fheads%2Fsov%2Fint 
ifeq ($(BUILDING_PIPELINE),)
	cd .. && ./ci/build.sh
else
	cd .. && ./ci/build-in-makefile.sh
endif
	cp ../toggles.yaml $@/package/
	cp ../toggles-diff.yaml $@/package/
	mkdir -p $@/package/bin
	cp ../bin/rollout $@/package/bin






