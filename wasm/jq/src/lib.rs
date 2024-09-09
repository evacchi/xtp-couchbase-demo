// Copyright 2024 Redpanda Data, Inc.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.
mod pdk;


use pdk::*;

use anyhow::{anyhow, ensure, Result};
use jaq_interpret::{Ctx, FilterT, ParseCtx, RcIter, Val};

use extism_pdk::*;

// Use the talc custom allocator for our Wasm binary, it's both faster and smaller than the default
// allocator that Rust uses for Wasm.
// See: https://github.com/SFBdragon/talc/blob/master/talc/README_WASM.md
//
// SAFETY: The runtime environment must be single-threaded WASM.
#[cfg(target_family = "wasm")]
#[global_allocator]
static ALLOCATOR: talc::TalckWasm = unsafe { talc::TalckWasm::new_global() };

pub(crate) fn transform(input: DataRecord) -> Result<(), Error> {    
    let mut defs = ParseCtx::new(Vec::new());
    defs.insert_natives(jaq_core::core());
    defs.insert_defs(jaq_std::std());
    assert!(defs.errs.is_empty()); // These are builtins it should always be valid.

    let filter_s = "del(.email)";

    let (f, errs) = jaq_parse::parse(&filter_s, jaq_parse::main());
    // TODO: report parse errors more gracefully
    ensure!(errs.is_empty(), "filter {filter_s} is invalid");
    let f = defs.compile(f.unwrap());
    ensure!(defs.errs.is_empty(), "filter {filter_s} is invalid");
    // Register our function that applies the jaq filter.


    let json_payload: serde_json::Value = serde_json::from_str(&input.doc)?;

    let inputs = RcIter::new(core::iter::empty());
    let ctx = Ctx::new([], &inputs);
    // Run the filter and write each JSON object to the output topic.

    for output in f.run((ctx, Val::from(json_payload))) {
        let value = output.map_err(|e| anyhow!("error: {e}"))?;
        let value: serde_json::Value = value.into();
        let value = serde_json::to_string(&value)?;

        emit(EmitRecord { 
            key: String::from(""), 
            value: value
        })?;
    }

    Ok(())
}
