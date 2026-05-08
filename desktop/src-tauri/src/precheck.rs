//! System-requirements precheck for Raven Local.
//!
//! Verifies the host meets the minimum thresholds (RAM, free disk, CPU
//! cores) before launching the compose stack. A failure surfaces a
//! structured event to the frontend so the splash can render an
//! actionable modal. Power users can bypass the check entirely with
//! `RAVEN_LOCAL_SKIP_REQS=true`.

use serde::Serialize;

/// Minimum RAM in gigabytes. Below this we warn the user.
pub const MIN_RAM_GB: u64 = 8;
/// Recommended RAM. Above this is comfortable for 7B–13B local models.
pub const RECOMMENDED_RAM_GB: u64 = 16;
/// Minimum free disk in gigabytes (for compose images + at least one model).
pub const MIN_FREE_DISK_GB: u64 = 20;
/// Minimum logical CPU cores.
pub const MIN_CPU_CORES: usize = 4;

/// Env var that bypasses the precheck for power users on edge configs.
pub const SKIP_ENV_VAR: &str = "RAVEN_LOCAL_SKIP_REQS";

#[derive(Clone, Debug, Serialize, PartialEq, Eq)]
pub struct PrecheckResult {
    pub ram_gb: u64,
    pub free_disk_gb: u64,
    pub cpu_cores: usize,
    pub ok: bool,
    pub warnings: Vec<Warning>,
}

#[derive(Clone, Debug, Serialize, PartialEq, Eq)]
#[serde(rename_all = "snake_case")]
pub enum Warning {
    LowRam { actual_gb: u64, minimum_gb: u64 },
    LowDisk { actual_gb: u64, minimum_gb: u64 },
    FewCpuCores { actual: usize, minimum: usize },
}

impl Warning {
    /// Short human-readable remediation text for the splash modal.
    pub fn remediation(&self) -> String {
        match self {
            Warning::LowRam { actual_gb, minimum_gb } => format!(
                "Detected {actual_gb} GB RAM (minimum {minimum_gb} GB). \
                 Close other applications, or choose a smaller Ollama model \
                 in onboarding."
            ),
            Warning::LowDisk { actual_gb, minimum_gb } => format!(
                "Only {actual_gb} GB free disk (minimum {minimum_gb} GB). \
                 Free up space — the largest consumer is usually downloaded \
                 model checkpoints under the data directory."
            ),
            Warning::FewCpuCores { actual, minimum } => format!(
                "Only {actual} CPU cores available (minimum {minimum}). \
                 Inference will be slow; consider a smaller model or a \
                 BYOK provider."
            ),
        }
    }
}

/// Provider abstraction so the threshold logic is unit-testable without
/// touching the real system.
pub trait SystemInfo {
    fn total_memory_gb(&self) -> u64;
    fn free_disk_gb(&self) -> u64;
    fn cpu_cores(&self) -> usize;
}

/// Real provider backed by `sysinfo` and the OS-level disk APIs.
pub struct RealSystemInfo;

impl SystemInfo for RealSystemInfo {
    fn total_memory_gb(&self) -> u64 {
        let mut sys = sysinfo::System::new();
        sys.refresh_memory();
        // sysinfo reports memory in bytes since 0.30+.
        let bytes = sys.total_memory();
        bytes / (1024 * 1024 * 1024)
    }

    fn free_disk_gb(&self) -> u64 {
        let disks = sysinfo::Disks::new_with_refreshed_list();
        // Sum free space across all mounted disks. Reasonable for a desktop
        // user who hasn't deliberately segregated volumes; the precheck
        // is a soft warning, not a hard quota.
        let bytes: u64 = disks.iter().map(|d| d.available_space()).sum();
        bytes / (1024 * 1024 * 1024)
    }

    fn cpu_cores(&self) -> usize {
        std::thread::available_parallelism()
            .map(|n| n.get())
            .unwrap_or(1)
    }
}

/// Run the precheck against the supplied provider. Returns the structured
/// result; the caller decides whether to honor `RAVEN_LOCAL_SKIP_REQS`.
pub fn run_precheck<S: SystemInfo>(sys: &S) -> PrecheckResult {
    let mut warnings = Vec::new();
    let ram_gb = sys.total_memory_gb();
    let free_disk_gb = sys.free_disk_gb();
    let cpu_cores = sys.cpu_cores();

    if ram_gb < MIN_RAM_GB {
        warnings.push(Warning::LowRam { actual_gb: ram_gb, minimum_gb: MIN_RAM_GB });
    }
    if free_disk_gb < MIN_FREE_DISK_GB {
        warnings.push(Warning::LowDisk { actual_gb: free_disk_gb, minimum_gb: MIN_FREE_DISK_GB });
    }
    if cpu_cores < MIN_CPU_CORES {
        warnings.push(Warning::FewCpuCores { actual: cpu_cores, minimum: MIN_CPU_CORES });
    }

    PrecheckResult {
        ram_gb,
        free_disk_gb,
        cpu_cores,
        ok: warnings.is_empty(),
        warnings,
    }
}

/// Returns true when the user has opted to bypass the precheck.
pub fn skip_requested() -> bool {
    std::env::var(SKIP_ENV_VAR)
        .map(|v| matches!(v.to_lowercase().as_str(), "1" | "true" | "yes"))
        .unwrap_or(false)
}

#[cfg(test)]
mod tests {
    use super::*;

    struct FakeSys {
        ram_gb: u64,
        disk_gb: u64,
        cpu: usize,
    }

    impl SystemInfo for FakeSys {
        fn total_memory_gb(&self) -> u64 { self.ram_gb }
        fn free_disk_gb(&self) -> u64 { self.disk_gb }
        fn cpu_cores(&self) -> usize { self.cpu }
    }

    #[test]
    fn passing_system_yields_ok_with_no_warnings() {
        let sys = FakeSys { ram_gb: 16, disk_gb: 100, cpu: 8 };
        let result = run_precheck(&sys);
        assert!(result.ok);
        assert!(result.warnings.is_empty());
        assert_eq!(result.ram_gb, 16);
        assert_eq!(result.free_disk_gb, 100);
        assert_eq!(result.cpu_cores, 8);
    }

    #[test]
    fn low_ram_yields_warning() {
        let sys = FakeSys { ram_gb: 4, disk_gb: 100, cpu: 8 };
        let result = run_precheck(&sys);
        assert!(!result.ok);
        assert_eq!(result.warnings.len(), 1);
        assert!(matches!(result.warnings[0], Warning::LowRam { actual_gb: 4, minimum_gb: 8 }));
    }

    #[test]
    fn low_disk_yields_warning() {
        let sys = FakeSys { ram_gb: 16, disk_gb: 5, cpu: 8 };
        let result = run_precheck(&sys);
        assert!(!result.ok);
        assert!(matches!(result.warnings[0], Warning::LowDisk { actual_gb: 5, minimum_gb: 20 }));
    }

    #[test]
    fn few_cores_yields_warning() {
        let sys = FakeSys { ram_gb: 16, disk_gb: 100, cpu: 2 };
        let result = run_precheck(&sys);
        assert!(!result.ok);
        assert!(matches!(result.warnings[0], Warning::FewCpuCores { actual: 2, minimum: 4 }));
    }

    #[test]
    fn multiple_warnings_collected() {
        let sys = FakeSys { ram_gb: 4, disk_gb: 5, cpu: 2 };
        let result = run_precheck(&sys);
        assert!(!result.ok);
        assert_eq!(result.warnings.len(), 3);
    }

    #[test]
    fn warnings_serialize_snake_case() {
        let w = Warning::LowRam { actual_gb: 4, minimum_gb: 8 };
        let json = serde_json::to_string(&w).unwrap();
        assert!(json.contains("\"low_ram\""), "got {json}");
        assert!(json.contains("\"actual_gb\":4"), "got {json}");
    }

    #[test]
    fn remediation_mentions_actual_and_minimum() {
        let w = Warning::LowRam { actual_gb: 4, minimum_gb: 8 };
        let text = w.remediation();
        assert!(text.contains("4 GB"), "got {text}");
        assert!(text.contains("8 GB"), "got {text}");
    }

    #[test]
    fn skip_requested_respects_env_var() {
        // Use an isolated subprocess-style approach by setting + unsetting.
        // Tests run in parallel by default — guard with a unique name to
        // avoid collision with other tests reading the same var.
        std::env::set_var(SKIP_ENV_VAR, "true");
        assert!(skip_requested());
        std::env::set_var(SKIP_ENV_VAR, "false");
        assert!(!skip_requested());
        std::env::set_var(SKIP_ENV_VAR, "1");
        assert!(skip_requested());
        std::env::remove_var(SKIP_ENV_VAR);
        assert!(!skip_requested());
    }
}
