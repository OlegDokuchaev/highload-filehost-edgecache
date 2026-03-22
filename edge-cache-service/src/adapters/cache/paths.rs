use std::path::{Path, PathBuf};

pub fn data_path(dir: &Path, id: &str) -> PathBuf {
    dir.join(format!("{id}.bin"))
}

pub fn meta_path(dir: &Path, id: &str) -> PathBuf {
    dir.join(format!("{id}.json"))
}

pub fn tmp_data_path(dir: &Path, id: &str) -> PathBuf {
    dir.join(format!("{id}.bin.tmp"))
}

pub fn tmp_meta_path(dir: &Path, id: &str) -> PathBuf {
    dir.join(format!("{id}.json.tmp"))
}
