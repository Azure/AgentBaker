// SPDX-License-Identifier: GPL-2.0-or-later
// Based on Debian/Ubuntu's update-ca-certificates.
// Copyright (c) 2003 Fumitoshi UKAI <ukai@debian.or.jp>
// Copyright (c) 2009 Philipp Kern <pkern@debian.org>

use std::collections::HashSet;
use std::env;
use std::error::Error;
use std::ffi::{OsStr, OsString};
use std::fs::{self, OpenOptions};
use std::io::{self, Write};
use std::os::unix::ffi::{OsStrExt, OsStringExt};
use std::os::unix::fs::{symlink, MetadataExt, OpenOptionsExt, PermissionsExt};
use std::path::{Path, PathBuf};
use std::process::{self, Command, Stdio};
use std::sync::atomic::{AtomicU64, Ordering};

type Result<T> = std::result::Result<T, Box<dyn Error>>;

struct Options {
    verbose: bool,
    fresh: bool,
    default: bool,
    config: PathBuf,
    system: PathBuf,
    local: PathBuf,
    store: PathBuf,
    hooks: PathBuf,
    bundle: PathBuf,
}

impl Options {
    fn parse() -> Result<Option<Self>> {
        let mut options = Self {
            verbose: false,
            fresh: false,
            default: false,
            config: "/etc/ca-certificates.conf".into(),
            system: "/usr/share/ca-certificates".into(),
            local: "/usr/local/share/ca-certificates".into(),
            store: "/etc/ssl/certs".into(),
            hooks: "/etc/ca-certificates/update.d".into(),
            bundle: "ca-certificates.crt".into(),
        };
        let mut args = env::args_os().skip(1);
        while let Some(arg) = args.next() {
            match arg.to_str() {
                Some("--verbose" | "-v") => options.verbose = true,
                Some("--fresh" | "-f") => options.fresh = true,
                Some("--default" | "-d") => {
                    options.default = true;
                    options.fresh = true;
                }
                Some("--help" | "-h") => {
                    println!(
                        "Usage: update-ca-certificates-rs [--verbose|-v] [--fresh|-f] \
                         [--default|-d]\n\
                         [--certsconf FILE] [--certsdir DIR] [--localcertsdir DIR]\n\
                         [--etccertsdir DIR] [--hooksdir DIR] [--certbundle FILE] [--sysroot DIR]"
                    );
                    return Ok(None);
                }
                Some(
                    "--certsconf" | "--certsdir" | "--localcertsdir" | "--etccertsdir"
                    | "--hooksdir" | "--certbundle" | "--sysroot",
                ) => {
                    let value = args
                        .next()
                        .filter(|value| !value.is_empty())
                        .ok_or_else(|| {
                            format!("{} requires a nonempty path", arg.to_string_lossy())
                        })?;
                    match arg.to_str() {
                        Some("--certsconf") => options.config = value.into(),
                        Some("--certsdir") => options.system = value.into(),
                        Some("--localcertsdir") => options.local = value.into(),
                        Some("--etccertsdir") => options.store = value.into(),
                        Some("--hooksdir") => options.hooks = value.into(),
                        Some("--certbundle") => options.bundle = value.into(),
                        Some("--sysroot") => {
                            for path in [
                                &mut options.config,
                                &mut options.system,
                                &mut options.local,
                                &mut options.store,
                                &mut options.hooks,
                            ] {
                                *path = append_path(Path::new(&value), path.as_os_str());
                            }
                        }
                        _ => unreachable!(),
                    }
                }
                _ => return Err(format!("Unknown option: {}", arg.to_string_lossy()).into()),
            }
        }
        let cwd = env::current_dir()?;
        for path in [
            &mut options.config,
            &mut options.system,
            &mut options.local,
            &mut options.store,
            &mut options.hooks,
        ] {
            if path.is_relative() {
                *path = cwd.join(&path);
            }
        }
        Ok(Some(options))
    }
}

fn at(path: &Path, error: io::Error) -> io::Error {
    io::Error::new(error.kind(), format!("{}: {error}", path.display()))
}

fn append_path(directory: &Path, name: &OsStr) -> PathBuf {
    let mut path = directory.as_os_str().to_os_string();
    path.push("/");
    path.push(name);
    path.into()
}

fn temporary_path(directory: &Path) -> PathBuf {
    static NEXT: AtomicU64 = AtomicU64::new(0);
    directory.join(format!(
        ".update-ca-certificates-rs-{}-{}",
        process::id(),
        NEXT.fetch_add(1, Ordering::Relaxed)
    ))
}

struct PendingPath(PathBuf);

impl Drop for PendingPath {
    fn drop(&mut self) {
        if let Err(error) = fs::remove_file(&self.0) {
            if error.kind() != io::ErrorKind::NotFound {
                eprintln!("W: cleaning up {}: {error}", self.0.display());
            }
        }
    }
}

fn walk(
    directory: &Path,
    follow_links: bool,
    ancestors: &mut HashSet<(u64, u64)>,
    output: &mut Vec<PathBuf>,
) -> Result<()> {
    let metadata = fs::metadata(directory).map_err(|error| at(directory, error))?;
    let identity = (metadata.dev(), metadata.ino());
    if !ancestors.insert(identity) {
        return Err(format!("Directory symlink loop: {}", directory.display()).into());
    }
    for entry in fs::read_dir(directory).map_err(|error| at(directory, error))? {
        let path = entry.map_err(|error| at(directory, error))?.path();
        let metadata = fs::symlink_metadata(&path).map_err(|error| at(&path, error))?;
        if !follow_links && metadata.is_symlink() {
            output.push(path);
            continue;
        }
        let metadata = if metadata.is_symlink() {
            match fs::metadata(&path) {
                Ok(metadata) => metadata,
                Err(error) if error.kind() == io::ErrorKind::NotFound => continue,
                Err(error) => return Err(at(&path, error).into()),
            }
        } else {
            metadata
        };
        if metadata.is_dir() {
            walk(&path, follow_links, ancestors, output)?;
        } else if follow_links && metadata.is_file() {
            output.push(path);
        }
    }
    ancestors.remove(&identity);
    Ok(())
}

fn entries(directory: &Path, follow_links: bool) -> Result<Vec<PathBuf>> {
    let mut paths = Vec::new();
    walk(directory, follow_links, &mut HashSet::new(), &mut paths)?;
    if follow_links {
        paths.retain(|path| path.as_os_str().as_bytes().ends_with(b".crt"));
        paths.sort_by(|left, right| {
            left.as_os_str()
                .as_bytes()
                .cmp(right.as_os_str().as_bytes())
        });
    }
    Ok(paths)
}

fn pem_name(certificate: &Path, normalize: bool) -> OsString {
    let name = certificate.file_name().unwrap_or_default().as_bytes();
    let name = if name.len() > 4 && name.ends_with(b".crt") {
        &name[..name.len() - 4]
    } else {
        name
    };
    let mut name = name.to_vec();
    if normalize {
        for byte in &mut name {
            *byte = match *byte {
                b' ' | b',' => b'_',
                b'(' | b')' => b'=',
                byte => byte,
            };
        }
    }
    name.extend_from_slice(b".pem");
    OsString::from_vec(name)
}

struct Update<'a> {
    options: &'a Options,
    added: Vec<PathBuf>,
    removed: Vec<PathBuf>,
    bundle: Vec<u8>,
    has_certificates: bool,
}

impl Update<'_> {
    fn add(&mut self, certificate: &Path) -> Result<()> {
        let pem = self.options.store.join(pem_name(certificate, true));
        if pem.as_os_str().as_bytes().contains(&b'\n') {
            return Err(format!("A hook path cannot contain a newline: {}", pem.display()).into());
        }
        let current_target = match fs::read_link(&pem) {
            Ok(target) => Some(target),
            Err(error)
                if matches!(
                    error.kind(),
                    io::ErrorKind::NotFound | io::ErrorKind::InvalidInput
                ) =>
            {
                None
            }
            Err(error) => return Err(at(&pem, error).into()),
        };
        if !pem.try_exists().map_err(|error| at(&pem, error))?
            || current_target.as_ref().map(|target| target.as_os_str())
                != Some(certificate.as_os_str())
        {
            let temporary = temporary_path(&self.options.store);
            symlink(certificate, &temporary).map_err(|error| at(&temporary, error))?;
            let temporary = PendingPath(temporary);
            fs::rename(&temporary.0, &pem).map_err(|error| at(&pem, error))?;
            self.added.push(pem);
        }
        let content = fs::read(certificate).map_err(|error| at(certificate, error))?;
        self.bundle.extend_from_slice(&content);
        if !content.is_empty() && !content.ends_with(b"\n") {
            self.bundle.push(b'\n');
        }
        self.has_certificates = true;
        Ok(())
    }

    fn remove(&mut self, certificate: &Path) -> Result<()> {
        // Match upstream's unnormalized basename on the removal path.
        let pem = self.options.store.join(pem_name(certificate, false));
        match fs::symlink_metadata(&pem) {
            Ok(metadata) if metadata.is_symlink() => {
                fs::remove_file(&pem).map_err(|error| at(&pem, error))?;
                self.removed.push(pem);
            }
            Ok(_) => {}
            Err(error) if error.kind() == io::ErrorKind::NotFound => {}
            Err(error) => return Err(at(&pem, error).into()),
        }
        Ok(())
    }

    fn publish_bundle(&self) -> Result<()> {
        if !self.has_certificates {
            return Ok(());
        }
        let destination = self.options.store.join(&self.options.bundle);
        let directory = destination
            .parent()
            .ok_or("Certificate bundle must have a parent directory")?;
        let temporary = temporary_path(directory);
        let mut file = OpenOptions::new()
            .write(true)
            .create_new(true)
            .mode(0o600)
            .open(&temporary)
            .map_err(|error| at(&temporary, error))?;
        let temporary = PendingPath(temporary);
        file.write_all(&self.bundle)?;
        file.set_permissions(fs::Permissions::from_mode(0o644))?;
        drop(file);
        fs::rename(&temporary.0, &destination).map_err(|error| at(&destination, error))?;
        let restorecon = Path::new("/sbin/restorecon");
        match fs::metadata(restorecon) {
            Ok(metadata) if metadata.permissions().mode() & 0o111 != 0 => {
                let output = Command::new(restorecon).arg(&destination).output()?;
                if !output.status.success() {
                    return Err(format!(
                        "restorecon failed ({}): {}",
                        output.status,
                        String::from_utf8_lossy(&output.stderr)
                    )
                    .into());
                }
            }
            Ok(_) => {}
            Err(error) if error.kind() == io::ErrorKind::NotFound => {}
            Err(error) => return Err(at(restorecon, error).into()),
        }
        Ok(())
    }

    fn hooks(&self) -> Result<()> {
        match fs::metadata(&self.options.hooks) {
            Ok(metadata) if metadata.is_dir() => {}
            Ok(_) => return Ok(()),
            Err(error) if error.kind() == io::ErrorKind::NotFound => return Ok(()),
            Err(error) => return Err(at(&self.options.hooks, error).into()),
        }
        println!("Running hooks in {}...", self.options.hooks.display());
        let mut command = Command::new("run-parts");
        if self.options.verbose {
            command.arg("--verbose");
        }
        let output = command
            .arg("--test")
            .arg("--")
            .arg(&self.options.hooks)
            .current_dir(&self.options.store)
            .output()?;
        io::stderr().write_all(&output.stderr)?;
        if !output.status.success() {
            return Err(format!("run-parts failed ({})", output.status).into());
        }
        let mut changes = Vec::new();
        for (prefix, paths) in [(b'+', &self.added), (b'-', &self.removed)] {
            for path in paths {
                changes.push(prefix);
                changes.extend_from_slice(path.as_os_str().as_bytes());
                changes.push(b'\n');
            }
        }
        for hook in output.stdout.split(|byte| *byte == b'\n') {
            if hook.is_empty() {
                continue;
            }
            let hook = Path::new(OsStr::from_bytes(hook));
            let spawn = |use_shell| {
                let mut command = if use_shell {
                    let mut shell = Command::new("/bin/sh");
                    shell.arg(hook);
                    shell
                } else {
                    Command::new(hook)
                };
                if self.options.fresh {
                    command.arg("--fresh");
                }
                command
                    .current_dir(&self.options.store)
                    .stdin(Stdio::piped())
                    .spawn()
            };
            let child = match spawn(false) {
                // Linux ENOEXEC: preserve the shell's handling of hooks without a shebang.
                Err(error) if error.raw_os_error() == Some(8) => spawn(true),
                result => result,
            };
            let mut child = match child {
                Ok(child) => child,
                Err(error) => {
                    eprintln!("E: could not execute hook {}: {error}", hook.display());
                    continue;
                }
            };
            let write_result = child
                .stdin
                .take()
                .ok_or("Hook stdin was not piped")?
                .write_all(&changes);
            let status = child.wait()?;
            if let Err(error) = write_result {
                if error.kind() != io::ErrorKind::BrokenPipe {
                    return Err(format!("Writing to hook {}: {error}", hook.display()).into());
                }
            }
            if !status.success() {
                println!(
                    "E: {} exited with code {}.",
                    hook.display(),
                    status
                        .code()
                        .map_or_else(|| "signal".to_string(), |code| code.to_string())
                );
            }
        }
        println!("done.");
        Ok(())
    }
}

fn trim_read(line: &[u8]) -> &[u8] {
    let start = line
        .iter()
        .position(|byte| !matches!(byte, b' ' | b'\t'))
        .unwrap_or(line.len());
    let end = line
        .iter()
        .rposition(|byte| !matches!(byte, b' ' | b'\t'))
        .map_or(start, |position| position + 1);
    &line[start..end]
}

fn run(mut options: Options) -> Result<()> {
    if !options.store.is_dir() {
        return Err(format!(
            "Certificate store is not a directory: {}",
            options.store.display()
        )
        .into());
    }
    let configuration = match fs::read(&options.config) {
        Ok(content) => content,
        Err(error) if error.kind() == io::ErrorKind::NotFound => {
            eprintln!(
                "W: {} not found; refreshing local certificates.",
                options.config.display()
            );
            Vec::new()
        }
        Err(error) => return Err(at(&options.config, error).into()),
    };
    options.fresh |= configuration.is_empty();
    if options.fresh {
        println!("Clearing symlinks in {}...", options.store.display());
        for link in entries(&options.store, false)? {
            let target = fs::read_link(&link).map_err(|error| at(&link, error))?;
            if target
                .as_os_str()
                .as_bytes()
                .starts_with(options.system.as_os_str().as_bytes())
                || target
                    .as_os_str()
                    .as_bytes()
                    .starts_with(options.local.as_os_str().as_bytes())
            {
                fs::remove_file(&link).map_err(|error| at(&link, error))?;
            }
        }
        for link in entries(&options.store, false)? {
            match fs::metadata(&link) {
                Ok(metadata) if metadata.is_file() => {}
                Ok(_) => fs::remove_file(&link).map_err(|error| at(&link, error))?,
                Err(error) if error.kind() == io::ErrorKind::NotFound => {
                    fs::remove_file(&link).map_err(|error| at(&link, error))?;
                }
                Err(error) => return Err(at(&link, error).into()),
            }
        }
        println!("done.");
    }
    println!("Updating certificates in {}...", options.store.display());
    let mut update = Update {
        options: &options,
        added: Vec::new(),
        removed: Vec::new(),
        bundle: Vec::new(),
        has_certificates: false,
    };
    if options.default {
        for certificate in entries(&options.system, true)? {
            update.add(&certificate)?;
        }
    }
    for line in configuration.split(|byte| *byte == b'\n') {
        if let Some(disabled) = line.strip_prefix(b"!") {
            update.remove(&append_path(
                &options.system,
                OsStr::from_bytes(trim_read(disabled)),
            ))?;
        }
    }
    for line in configuration.split(|byte| *byte == b'\n') {
        if line.is_empty() || matches!(line[0], b'#' | b'!') {
            continue;
        }
        let certificate = append_path(&options.system, OsStr::from_bytes(trim_read(line)));
        match fs::metadata(&certificate) {
            Ok(metadata) if metadata.is_file() => update.add(&certificate)?,
            Ok(_) => eprintln!(
                "W: {} not found, but listed in {}.",
                certificate.display(),
                options.config.display()
            ),
            Err(error) if error.kind() == io::ErrorKind::NotFound => eprintln!(
                "W: {} not found, but listed in {}.",
                certificate.display(),
                options.config.display()
            ),
            Err(error) => return Err(at(&certificate, error).into()),
        }
    }
    match fs::metadata(&options.local) {
        Ok(metadata) if metadata.is_dir() => {
            for certificate in entries(&options.local, true)? {
                update.add(&certificate)?;
            }
        }
        Ok(_) => {}
        Err(error) if error.kind() == io::ErrorKind::NotFound => {}
        Err(error) => return Err(at(&options.local, error).into()),
    }
    if !update.added.is_empty() || !update.removed.is_empty() {
        for link in entries(&options.store, false)? {
            if !link.try_exists().map_err(|error| at(&link, error))? {
                fs::remove_file(&link).map_err(|error| at(&link, error))?;
                if options.verbose {
                    println!("Removed orphan symlink {}", link.display());
                }
            }
        }
        let mut command = Command::new("openssl");
        command.arg("rehash");
        if options.verbose {
            command.arg("-v");
        } else {
            command.stdout(Stdio::null());
        }
        let status = command.arg(".").current_dir(&options.store).status()?;
        if !status.success() {
            return Err(format!("openssl rehash failed ({status})").into());
        }
    }
    update.publish_bundle()?;
    println!(
        "{} added, {} removed; done.",
        update.added.len(),
        update.removed.len()
    );
    update.hooks()
}

fn main() {
    let result = Options::parse().and_then(|options| match options {
        Some(options) => run(options),
        None => Ok(()),
    });
    if let Err(error) = result {
        eprintln!("update-ca-certificates-rs: {error}");
        process::exit(1);
    }
}
