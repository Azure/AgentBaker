package gpu

/* ConvergedGPUDriverSizes : these sizes use a "converged" driver to support both cuda/grid workloads.
see https://github.com/Azure/azhpc-extensions/blob/daaefd78df6f27012caf30f3b54c3bd6dc437652/NvidiaGPU/resources.json
*/
//nolint:gochecknoglobals
var ConvergedGPUDriverSizes = map[string]bool{
	"standard_nv6ads_a10_v5":   true,
	"standard_nv12ads_a10_v5":  true,
	"standard_nv18ads_a10_v5":  true,
	"standard_nv36ads_a10_v5":  true,
	"standard_nv72ads_a10_v5":  true,
	"standard_nv36adms_a10_v5": true,
	"standard_nc8ads_a10_v4":   true,
	"standard_nc16ads_a10_v4":  true,
	"standard_nc32ads_a10_v4":  true,
}

// RTXPro6000GPUDriverSizes : SKUs requiring the GRID v20 (595.x) driver.
//
//nolint:gochecknoglobals
var RTXPro6000GPUDriverSizes = map[string]bool{
	"standard_nc24lds_xl_rtxpro6000bse_v6":  true,
	"standard_nc36ds_xl_rtxpro6000bse_v6":   true,
	"standard_nc36lds_xl_rtxpro6000bse_v6":  true,
	"standard_nc72ds_xl_rtxpro6000bse_v6":   true,
	"standard_nc72lds_xl_rtxpro6000bse_v6":  true,
	"standard_nc144ds_xl_rtxpro6000bse_v6":  true,
	"standard_nc144lds_xl_rtxpro6000bse_v6": true,
	"standard_nc288ds_xl_rtxpro6000bse_v6":  true,
	"standard_nc288lds_xl_rtxpro6000bse_v6": true,
	// Preview sizes retained as backward-compat aliases.
	"standard_nc128ds_xl_rtxpro6000bse_v6":  true,
	"standard_nc128lds_xl_rtxpro6000bse_v6": true,
	"standard_nc256ds_xl_rtxpro6000bse_v6":  true,
	"standard_nc256lds_xl_rtxpro6000bse_v6": true,
	"standard_nc320ds_xl_rtxpro6000bse_v6":  true,
	"standard_nc320lds_xl_rtxpro6000bse_v6": true,
}

//nolint:gochecknoglobals
var FabricManagerGPUSizes = map[string]bool{
	// A100
	"standard_nd96asr_v4":        true,
	"standard_nd112asr_a100_v4":  true,
	"standard_nd120asr_a100_v4":  true,
	"standard_nd96amsr_a100_v4":  true,
	"standard_nd112amsr_a100_v4": true,
	"standard_nd120amsr_a100_v4": true,
	"standard_nd96ams_a100_v4":   true,
	"standard_nd96ams_v4":        true,
	// H100
	"standard_nd46s_h100_v5":    true,
	"standard_nd48s_h100_v5":    true,
	"standard_nd50s_h100_v5":    true,
	"standard_nd92is_h100_v5":   true,
	"standard_nd96is_h100_v5":   true,
	"standard_nd100is_h100_v5":  true,
	"standard_nd92isr_h100_v5":  true,
	"standard_nd96isr_h100_v5":  true,
	"standard_nd100isr_h100_v5": true,
	// H200
	"standard_nd96is_h200_v5":   true,
	"standard_nd96isr_h200_v5":  true,
	"standard_nd96isrf_h200_v5": true,
	// A100 oddballs.
	"standard_nc24ads_a100_v4": false,
	"standard_nc48ads_a100_v4": false,
	"standard_nc96ads_a100_v4": false,
}
