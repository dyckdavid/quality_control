use super::error::ApiError;
use super::models::*;
use serde::de::DeserializeOwned;
use serde::Serialize;
use std::collections::HashMap;
use std::io::Read;
use uuid::Uuid;

/// HTTP client for the quality-control JSON API.
/// Callers should invoke methods off the UI thread (e.g. via `poll_promise`).
#[derive(Clone, Debug)]
pub struct ApiClient {
    base_url: String,
    access_token: Option<String>,
    agent: ureq::Agent,
}

impl ApiClient {
    pub fn new(base_url: impl Into<String>) -> Self {
        let agent = ureq::AgentBuilder::new()
            .timeout_connect(std::time::Duration::from_secs(5))
            .timeout_read(std::time::Duration::from_secs(30))
            .build();
        Self {
            base_url: trim_slash(base_url.into()),
            access_token: None,
            agent,
        }
    }

    pub fn base_url(&self) -> &str {
        &self.base_url
    }

    pub fn set_base_url(&mut self, url: impl Into<String>) {
        self.base_url = trim_slash(url.into());
    }

    pub fn set_access_token(&mut self, token: Option<String>) {
        self.access_token = token;
    }

    pub fn access_token(&self) -> Option<&str> {
        self.access_token.as_deref()
    }

    pub fn healthz(&self) -> Result<(), ApiError> {
        let url = format!("{}/healthz", self.base_url);
        let resp = self.agent.get(&url).call().map_err(transport)?;
        if resp.status() >= 200 && resp.status() < 300 {
            Ok(())
        } else {
            Err(ApiError::Http {
                status: resp.status(),
                code: "health".into(),
                message: "health check failed".into(),
            })
        }
    }

    pub fn version(&self) -> Result<VersionInfo, ApiError> {
        self.get_raw("/api/v1/version")
    }

    pub fn login(&mut self, username: &str, password: &str) -> Result<TokenResponse, ApiError> {
        let body = LoginRequest {
            username: username.to_string(),
            password: password.to_string(),
        };
        let session: TokenResponse = self.post_data("/api/v1/auth/login", &body)?;
        self.access_token = Some(session.access_token.clone());
        Ok(session)
    }

    pub fn refresh(&self, refresh_token: &str) -> Result<TokenResponse, ApiError> {
        let body = RefreshRequest {
            refresh_token: refresh_token.to_string(),
        };
        self.post_data("/api/v1/auth/refresh", &body)
    }

    pub fn logout(&self, refresh_token: &str) -> Result<(), ApiError> {
        let body = RefreshRequest {
            refresh_token: refresh_token.to_string(),
        };
        let _: serde_json::Value = self.request_json("POST", "/api/v1/auth/logout", Some(&body), &HashMap::new())?;
        Ok(())
    }

    pub fn get_plan(&self, id: Uuid) -> Result<PlanDetail, ApiError> {
        self.get_data(&format!("/api/v1/plans/{id}"))
    }

    pub fn get_lot(&self, id: Uuid) -> Result<Lot, ApiError> {
        self.get_data(&format!("/api/v1/lots/{id}"))
    }

    pub fn set_lot_status(&self, id: Uuid, status: &str) -> Result<Lot, ApiError> {
        self.post_data(
            &format!("/api/v1/lots/{id}/status"),
            &serde_json::json!({ "status": status }),
        )
    }

    pub fn list_defect_codes(&self) -> Result<Vec<DefectCode>, ApiError> {
        let mut q = HashMap::new();
        q.insert("limit", "200".to_string());
        q.insert("offset", "0".to_string());
        self.get_page("/api/v1/defect-codes", &q).map(|(rows, _)| rows)
    }

    pub fn list_gauges(&self) -> Result<Vec<Gauge>, ApiError> {
        let mut q = HashMap::new();
        q.insert("limit", "200".to_string());
        q.insert("offset", "0".to_string());
        self.get_page("/api/v1/gauges", &q).map(|(rows, _)| rows)
    }

    pub fn record_results(&self, id: Uuid, body: &ResultBatch) -> Result<serde_json::Value, ApiError> {
        self.post_data(&format!("/api/v1/inspections/{id}/results"), body)
    }

    pub fn add_defect(&self, id: Uuid, body: &NewInspectionDefect) -> Result<serde_json::Value, ApiError> {
        self.post_data(&format!("/api/v1/inspections/{id}/defects"), body)
    }

    pub fn complete_inspection(
        &self,
        id: Uuid,
        body: &CompleteInspection,
    ) -> Result<CompletedInspection, ApiError> {
        self.post_data(&format!("/api/v1/inspections/{id}/complete"), body)
    }

    pub fn cancel_inspection(&self, id: Uuid) -> Result<Inspection, ApiError> {
        self.post_data(&format!("/api/v1/inspections/{id}/cancel"), &serde_json::json!({}))
    }

    pub fn dispose_ncr(&self, id: Uuid, body: &DispositionRequest) -> Result<Nonconformance, ApiError> {
        self.post_data(&format!("/api/v1/nonconformances/{id}/disposition"), body)
    }

    pub fn close_ncr(&self, id: Uuid) -> Result<Nonconformance, ApiError> {
        self.post_data(&format!("/api/v1/nonconformances/{id}/close"), &serde_json::json!({}))
    }

    pub fn void_ncr(&self, id: Uuid, notes: &str) -> Result<Nonconformance, ApiError> {
        self.post_data(
            &format!("/api/v1/nonconformances/{id}/void"),
            &NoteBody { notes: notes.to_string() },
        )
    }

    /// Moves a corrective action. `action` is one of start, verify, close, reopen, cancel.
    pub fn move_capa(&self, id: Uuid, action: &str, notes: &str) -> Result<CorrectiveAction, ApiError> {
        self.post_data(
            &format!("/api/v1/corrective-actions/{id}/{action}"),
            &NoteBody { notes: notes.to_string() },
        )
    }

    pub fn me(&self) -> Result<User, ApiError> {
        self.get_data("/api/v1/me")
    }

    pub fn dashboard(&self) -> Result<Dashboard, ApiError> {
        self.get_data("/api/v1/dashboard")
    }

    pub fn list_inspections(
        &self,
        status: Option<&str>,
        limit: i32,
        offset: i32,
    ) -> Result<(Vec<Inspection>, Page), ApiError> {
        let mut q = HashMap::new();
        q.insert("limit", limit.to_string());
        q.insert("offset", offset.to_string());
        if let Some(s) = status {
            if !s.is_empty() {
                q.insert("status", s.to_string());
            }
        }
        self.get_page("/api/v1/inspections", &q)
    }

    pub fn get_inspection(&self, id: Uuid) -> Result<InspectionDetail, ApiError> {
        self.get_data(&format!("/api/v1/inspections/{id}"))
    }

    pub fn create_inspection(&self, body: &NewInspection) -> Result<InspectionDetail, ApiError> {
        self.post_data("/api/v1/inspections", body)
    }

    pub fn list_ncrs(
        &self,
        status: Option<&str>,
        limit: i32,
        offset: i32,
    ) -> Result<(Vec<Nonconformance>, Page), ApiError> {
        let mut q = HashMap::new();
        q.insert("limit", limit.to_string());
        q.insert("offset", offset.to_string());
        if let Some(s) = status {
            if !s.is_empty() {
                q.insert("status", s.to_string());
            }
        }
        self.get_page("/api/v1/nonconformances", &q)
    }

    pub fn get_ncr(&self, id: Uuid) -> Result<NcrDetail, ApiError> {
        self.get_data(&format!("/api/v1/nonconformances/{id}"))
    }

    pub fn create_ncr(&self, body: &NewNonconformance) -> Result<Nonconformance, ApiError> {
        self.post_data("/api/v1/nonconformances", body)
    }

    pub fn update_ncr(&self, id: Uuid, body: &PatchNonconformance) -> Result<Nonconformance, ApiError> {
        self.patch_data(&format!("/api/v1/nonconformances/{id}"), body)
    }

    pub fn list_capas(
        &self,
        status: Option<&str>,
        limit: i32,
        offset: i32,
    ) -> Result<(Vec<CorrectiveAction>, Page), ApiError> {
        let mut q = HashMap::new();
        q.insert("limit", limit.to_string());
        q.insert("offset", offset.to_string());
        if let Some(s) = status {
            if !s.is_empty() {
                q.insert("status", s.to_string());
            }
        }
        self.get_page("/api/v1/corrective-actions", &q)
    }

    pub fn get_capa(&self, id: Uuid) -> Result<CorrectiveAction, ApiError> {
        self.get_data(&format!("/api/v1/corrective-actions/{id}"))
    }

    pub fn create_capa(&self, body: &NewCorrectiveAction) -> Result<CorrectiveAction, ApiError> {
        self.post_data("/api/v1/corrective-actions", body)
    }

    pub fn update_capa(
        &self,
        id: Uuid,
        body: &PatchCorrectiveAction,
    ) -> Result<CorrectiveAction, ApiError> {
        self.patch_data(&format!("/api/v1/corrective-actions/{id}"), body)
    }

    pub fn list_products(&self, q: Option<&str>) -> Result<(Vec<Product>, Page), ApiError> {
        let mut params = HashMap::new();
        params.insert("limit", "200".to_string());
        params.insert("offset", "0".to_string());
        if let Some(q) = q {
            if !q.is_empty() {
                params.insert("q", q.to_string());
            }
        }
        self.get_page("/api/v1/products", &params)
    }

    pub fn list_lots(&self, q: Option<&str>) -> Result<(Vec<Lot>, Page), ApiError> {
        let mut params = HashMap::new();
        params.insert("limit", "200".to_string());
        params.insert("offset", "0".to_string());
        if let Some(q) = q {
            if !q.is_empty() {
                params.insert("q", q.to_string());
            }
        }
        self.get_page("/api/v1/lots", &params)
    }

    pub fn list_sites(&self) -> Result<(Vec<Site>, Page), ApiError> {
        let mut params = HashMap::new();
        params.insert("limit", "200".to_string());
        params.insert("offset", "0".to_string());
        self.get_page("/api/v1/sites", &params)
    }

    pub fn create_site(&self, body: &NewSite) -> Result<Site, ApiError> {
        self.post_data("/api/v1/sites", body)
    }

    pub fn create_product(&self, body: &NewProduct) -> Result<Product, ApiError> {
        self.post_data("/api/v1/products", body)
    }

    pub fn create_lot(&self, body: &NewLot) -> Result<Lot, ApiError> {
        self.post_data("/api/v1/lots", body)
    }

    /// Lists any paged collection as JSON objects so every plant record can be managed.
    pub fn list_json(&self, path: &str) -> Result<Vec<serde_json::Value>, ApiError> {
        let mut params = HashMap::new();
        params.insert("limit", "200".to_string());
        params.insert("offset", "0".to_string());
        let (rows, _page): (Vec<serde_json::Value>, Page) = self.get_page(path, &params)?;
        Ok(rows)
    }

    pub fn list_json_query(
        &self,
        path: &str,
        extra: &[(&str, String)],
    ) -> Result<Vec<serde_json::Value>, ApiError> {
        let mut params = HashMap::new();
        params.insert("limit", "200".to_string());
        params.insert("offset", "0".to_string());
        for (k, v) in extra {
            if !v.is_empty() {
                params.insert(*k, v.clone());
            }
        }
        let (rows, _page): (Vec<serde_json::Value>, Page) = self.get_page(path, &params)?;
        Ok(rows)
    }

    pub fn get_json(&self, path: &str) -> Result<serde_json::Value, ApiError> {
        self.get_data(path)
    }

    pub fn get_list_envelope(
        &self,
        path: &str,
        extra: &[(&str, String)],
    ) -> Result<Vec<serde_json::Value>, ApiError> {
        let mut params = HashMap::new();
        params.insert("limit", "100".to_string());
        for (k, v) in extra {
            if !v.is_empty() {
                params.insert(*k, v.clone());
            }
        }
        let env: DataEnvelope<Vec<serde_json::Value>> =
            self.request_json("GET", path, None::<&()>, &params)?;
        Ok(env.data)
    }

    pub fn post_json(&self, path: &str, body: &serde_json::Value) -> Result<serde_json::Value, ApiError> {
        self.post_data(path, body)
    }

    pub fn patch_json(&self, path: &str, body: &serde_json::Value) -> Result<serde_json::Value, ApiError> {
        self.patch_data(path, body)
    }

    pub fn delete_path(&self, path: &str) -> Result<(), ApiError> {
        let url = format!("{}{}", self.base_url, path);
        let mut req = self.agent.delete(&url).set("Accept", "application/json");
        if let Some(token) = &self.access_token {
            req = req.set("Authorization", &format!("Bearer {token}"));
        }
        match req.call() {
            Ok(resp) if (200..300).contains(&resp.status()) => Ok(()),
            Ok(resp) => Err(ApiError::Http {
                status: resp.status(),
                code: "http".into(),
                message: "delete failed".into(),
            }),
            Err(ureq::Error::Status(status, resp)) => {
                let text = resp.into_string().unwrap_or_default();
                Err(parse_error(status, &text))
            }
            Err(e) => Err(ApiError::Transport(e.to_string())),
        }
    }

    pub fn download(&self, path: &str) -> Result<Vec<u8>, ApiError> {
        let url = format!("{}{}", self.base_url, path);
        let mut req = self.agent.get(&url);
        if let Some(token) = &self.access_token {
            req = req.set("Authorization", &format!("Bearer {token}"));
        }
        match req.call() {
            Ok(resp) => {
                let mut buf = Vec::new();
                resp.into_reader()
                    .read_to_end(&mut buf)
                    .map_err(|e| ApiError::Transport(e.to_string()))?;
                Ok(buf)
            }
            Err(ureq::Error::Status(status, resp)) => {
                let text = resp.into_string().unwrap_or_default();
                Err(parse_error(status, &text))
            }
            Err(e) => Err(ApiError::Transport(e.to_string())),
        }
    }

    fn get_data<T: DeserializeOwned>(&self, path: &str) -> Result<T, ApiError> {
        let env: DataEnvelope<T> = self.request_json("GET", path, None::<&()>, &HashMap::new())?;
        Ok(env.data)
    }

    fn get_raw<T: DeserializeOwned>(&self, path: &str) -> Result<T, ApiError> {
        self.request_json("GET", path, None::<&()>, &HashMap::new())
    }

    fn get_page<T: DeserializeOwned>(
        &self,
        path: &str,
        query: &HashMap<&str, String>,
    ) -> Result<(T, Page), ApiError> {
        let env: PageEnvelope<T> = self.request_json("GET", path, None::<&()>, query)?;
        Ok((env.data, env.page))
    }

    fn post_data<B: Serialize, T: DeserializeOwned>(
        &self,
        path: &str,
        body: &B,
    ) -> Result<T, ApiError> {
        let env: DataEnvelope<T> = self.request_json("POST", path, Some(body), &HashMap::new())?;
        Ok(env.data)
    }

    fn patch_data<B: Serialize, T: DeserializeOwned>(
        &self,
        path: &str,
        body: &B,
    ) -> Result<T, ApiError> {
        let env: DataEnvelope<T> = self.request_json("PATCH", path, Some(body), &HashMap::new())?;
        Ok(env.data)
    }

    fn request_json<B: Serialize, T: DeserializeOwned>(
        &self,
        method: &str,
        path: &str,
        body: Option<&B>,
        query: &HashMap<&str, String>,
    ) -> Result<T, ApiError> {
        let mut url = format!("{}{}", self.base_url, path);
        if !query.is_empty() {
            let qs: Vec<String> = query
                .iter()
                .map(|(k, v)| format!("{}={}", k, urlencoding(v)))
                .collect();
            url.push('?');
            url.push_str(&qs.join("&"));
        }

        let mut req = match method {
            "GET" => self.agent.get(&url),
            "POST" => self.agent.post(&url),
            "PATCH" => self.agent.request("PATCH", &url),
            "DELETE" => self.agent.delete(&url),
            other => {
                return Err(ApiError::Invalid(format!("unsupported method {other}")));
            }
        };
        req = req.set("Accept", "application/json");
        if let Some(token) = &self.access_token {
            req = req.set("Authorization", &format!("Bearer {token}"));
        }

        let result = if let Some(b) = body {
            req.set("Content-Type", "application/json")
                .send_json(b)
        } else if method == "GET" || method == "DELETE" {
            req.call()
        } else {
            req.set("Content-Type", "application/json")
                .send_string("{}")
        };

        match result {
            Ok(resp) => {
                let status = resp.status();
                let text = resp.into_string().map_err(|e| ApiError::Transport(e.to_string()))?;
                if !(200..300).contains(&status) {
                    return Err(parse_error(status, &text));
                }
                let text = if text.trim().is_empty() { "null".to_string() } else { text };
                serde_json::from_str(&text).map_err(|e| ApiError::Decode(e.to_string()))
            }
            Err(ureq::Error::Status(status, resp)) => {
                let text = resp
                    .into_string()
                    .unwrap_or_else(|_| String::from("request failed"));
                Err(parse_error(status, &text))
            }
            Err(e) => Err(ApiError::Transport(e.to_string())),
        }
    }
}

fn parse_error(status: u16, text: &str) -> ApiError {
    if let Ok(env) = serde_json::from_str::<ErrorEnvelope>(text) {
        ApiError::Http {
            status,
            code: env.error.code,
            message: env.error.message,
        }
    } else {
        ApiError::Http {
            status,
            code: "http".into(),
            message: text.chars().take(200).collect(),
        }
    }
}

fn transport(err: ureq::Error) -> ApiError {
    match err {
        ureq::Error::Status(status, resp) => {
            let text = resp
                .into_string()
                .unwrap_or_else(|_| String::from("request failed"));
            parse_error(status, &text)
        }
        other => ApiError::Transport(other.to_string()),
    }
}

/// Also drops a pasted `/api/v1`, since every request path already starts with it.
fn trim_slash(s: String) -> String {
    let mut s = s.trim().trim_end_matches('/').to_string();
    if s.ends_with("/api/v1") {
        s.truncate(s.len() - "/api/v1".len());
    }
    s.trim_end_matches('/').to_string()
}

fn urlencoding(s: &str) -> String {
    let mut out = String::with_capacity(s.len());
    for b in s.bytes() {
        match b {
            b'A'..=b'Z' | b'a'..=b'z' | b'0'..=b'9' | b'-' | b'_' | b'.' | b'~' => {
                out.push(b as char);
            }
            _ => out.push_str(&format!("%{b:02X}")),
        }
    }
    out
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn trim_slash_removes_trailing() {
        assert_eq!(trim_slash("http://localhost:8080/".into()), "http://localhost:8080");
        assert_eq!(trim_slash(" http://shop:8080/api/v1/ ".into()), "http://shop:8080");
    }

    #[test]
    fn urlencoding_encodes_spaces() {
        assert_eq!(urlencoding("a b"), "a%20b");
    }

    #[test]
    fn parse_error_reads_api_shape() {
        let err = parse_error(
            401,
            r#"{"error":{"code":"unauthorized","message":"Sign in is required."}}"#,
        );
        match err {
            ApiError::Http { status, code, message } => {
                assert_eq!(status, 401);
                assert_eq!(code, "unauthorized");
                assert!(message.contains("Sign in"));
            }
            other => panic!("unexpected {other:?}"),
        }
    }
}
