#![cfg_attr(not(debug_assertions), windows_subsystem = "windows")]

use std::process::{Child, Command};
use std::sync::Mutex;
use serde::{Deserialize, Serialize};

#[derive(Default)]
struct NodeState {
    process: Option<Child>,
}

struct AppState {
    node: Mutex<NodeState>,
}

#[derive(Serialize, Deserialize, Clone)]
struct NodeStatus {
    running: bool,
    pid: Option<u32>,
}

#[derive(Serialize, Deserialize, Clone)]
struct NodeConfig {
    platform_url: String,
    node_id: String,
    node_secret: String,
}

#[tauri::command]
fn start_node(state: tauri::State<AppState>, config: NodeConfig) -> Result<NodeStatus, String> {
    let mut ns = state.node.lock().map_err(|e| e.to_string())?;
    
    // Kill existing process if any
    if let Some(ref mut child) = ns.process {
        let _ = child.kill();
    }

    // Find node binary next to the app, or in PATH
    let node_path = std::env::current_exe()
        .map(|p| p.parent().unwrap().join("node").join("node"))
        .unwrap_or_else(|_| "node".into());

    let child = Command::new(&node_path)
        .spawn()
        .map_err(|e| format!("Failed to start node: {}", e))?;

    let pid = child.id();
    ns.process = Some(child);

    // Store config for future restarts
    let _ = config;

    Ok(NodeStatus {
        running: true,
        pid,
    })
}

#[tauri::command]
fn stop_node(state: tauri::State<AppState>) -> Result<NodeStatus, String> {
    let mut ns = state.node.lock().map_err(|e| e.to_string())?;
    if let Some(ref mut child) = ns.process {
        let _ = child.kill();
    }
    ns.process = None;
    Ok(NodeStatus {
        running: false,
        pid: None,
    })
}

#[tauri::command]
fn node_status(state: tauri::State<AppState>) -> Result<NodeStatus, String> {
    let mut ns = state.node.lock().map_err(|e| e.to_string())?;
    if let Some(ref mut child) = ns.process {
        match child.try_wait() {
            Ok(Some(_)) => {
                ns.process = None;
                Ok(NodeStatus { running: false, pid: None })
            }
            Ok(None) => Ok(NodeStatus { running: true, pid: Some(child.id()) }),
            Err(e) => Err(format!("Status check failed: {}", e)),
        }
    } else {
        Ok(NodeStatus { running: false, pid: None })
    }
}

fn main() {
    tauri::Builder::default()
        .plugin(tauri_plugin_shell::init())
        .manage(AppState {
            node: Mutex::new(NodeState::default()),
        })
        .invoke_handler(tauri::generate_handler![
            start_node,
            stop_node,
            node_status,
        ])
        .run(tauri::generate_context!())
        .expect("error while running tauri application");
}
