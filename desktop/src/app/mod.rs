mod theme;
mod widgets;

use crate::api::{
    ApiClient, ApiError, CorrectiveAction, Dashboard, Inspection, InspectionDetail, Lot,
    NcrDetail, NewCorrectiveAction, NewInspection, NewNonconformance, Nonconformance, Product,
    TokenResponse,
};
use crate::csv_export;
use crate::settings::{AppSettings, ThemePreference};
use eframe::egui;
use poll_promise::Promise;
use std::path::PathBuf;
use uuid::Uuid;

use theme::apply_theme;
use widgets::{empty_state, error_banner, loading_spinner, searchable_field, status_badge};

#[derive(Debug, Clone, Copy, PartialEq, Eq)]
pub enum NavPage {
    Dashboard,
    Inspections,
    Nonconformances,
    CorrectiveActions,
    Settings,
}

impl NavPage {
    fn label(self) -> &'static str {
        match self {
            NavPage::Dashboard => "Dashboard",
            NavPage::Inspections => "Inspections",
            NavPage::Nonconformances => "Nonconformances",
            NavPage::CorrectiveActions => "Corrective Actions",
            NavPage::Settings => "Settings",
        }
    }

    fn all() -> &'static [NavPage] {
        &[
            NavPage::Dashboard,
            NavPage::Inspections,
            NavPage::Nonconformances,
            NavPage::CorrectiveActions,
            NavPage::Settings,
        ]
    }
}

#[derive(Debug, Clone, Copy, PartialEq, Eq)]
enum SortKey {
    Number,
    Status,
    Result,
    Severity,
    Title,
    Due,
    Opened,
}

#[derive(Default)]
struct SortState {
    key: Option<SortKey>,
    asc: bool,
}

enum DetailView {
    None,
    Inspection(Uuid),
    Ncr(Uuid),
    Capa(Uuid),
}

enum FormMode {
    None,
    CreateInspection,
    CreateNcr,
    CreateCapa,
    EditNcr(Uuid),
    EditCapa(Uuid),
}

struct LoginForm {
    username: String,
    password: String,
    error: Option<String>,
}

struct InspectionForm {
    lot_id: String,
    notes: String,
    error: Option<String>,
}

struct NcrForm {
    product_id: String,
    title: String,
    description: String,
    severity: String,
    quantity: String,
    error: Option<String>,
}

struct CapaForm {
    title: String,
    root_cause: String,
    action_plan: String,
    due_date: String,
    error: Option<String>,
}

#[derive(Debug, Clone, Copy, PartialEq, Eq)]
enum ShotStep {
    WaitLogin,
    CaptureLogin,
    WaitDashboard,
    CaptureDashboard,
    CaptureInspections,
    CaptureNcrs,
    CaptureCapas,
    CaptureSettings,
    Done,
}

pub struct LaunchOptions {
    pub settings: AppSettings,
    pub screenshot_dir: Option<PathBuf>,
    pub auto_login_user: Option<String>,
    pub auto_login_pass: Option<String>,
}

pub struct QcApp {
    settings: AppSettings,
    client: ApiClient,
    session: Option<TokenResponse>,
    page: NavPage,
    login: LoginForm,
    status_msg: Option<String>,

    dashboard: Option<Dashboard>,
    inspections: Vec<Inspection>,
    ncrs: Vec<Nonconformance>,
    capas: Vec<CorrectiveAction>,
    products: Vec<Product>,
    lots: Vec<Lot>,

    search: String,
    status_filter: String,
    sort: SortState,
    detail: DetailView,
    form: FormMode,
    insp_form: InspectionForm,
    ncr_form: NcrForm,
    capa_form: CapaForm,

    inspection_detail: Option<InspectionDetail>,
    ncr_detail: Option<NcrDetail>,
    capa_detail: Option<CorrectiveAction>,

    login_promise: Option<Promise<Result<TokenResponse, ApiError>>>,
    dash_promise: Option<Promise<Result<Dashboard, ApiError>>>,
    list_promise: Option<Promise<Result<ListPayload, ApiError>>>,
    detail_promise: Option<Promise<Result<DetailPayload, ApiError>>>,
    mutate_promise: Option<Promise<Result<MutatePayload, ApiError>>>,
    bootstrap_promise: Option<Promise<Result<BootstrapPayload, ApiError>>>,

    loading: bool,
    list_error: Option<String>,

    screenshot_dir: Option<PathBuf>,
    shot_step: Option<ShotStep>,
    shot_frames: u32,
    pending_shot_name: Option<String>,
    should_close: bool,
}

enum ListPayload {
    Inspections(Vec<Inspection>),
    Ncrs(Vec<Nonconformance>),
    Capas(Vec<CorrectiveAction>),
}

enum DetailPayload {
    Inspection(InspectionDetail),
    Ncr(NcrDetail),
    Capa(CorrectiveAction),
}

enum MutatePayload {
    Ok(String),
    InspectionCreated(InspectionDetail),
    NcrSaved(Nonconformance),
    CapaSaved(CorrectiveAction),
}

struct BootstrapPayload {
    products: Vec<Product>,
    lots: Vec<Lot>,
}

impl QcApp {
    pub fn new(cc: &eframe::CreationContext<'_>, opts: LaunchOptions) -> Self {
        apply_theme(&cc.egui_ctx, opts.settings.theme);
        egui_extras::install_image_loaders(&cc.egui_ctx);

        let client = ApiClient::new(opts.settings.server_url.clone());

        Self {
            login: LoginForm {
                username: opts
                    .auto_login_user
                    .clone()
                    .unwrap_or_else(|| opts.settings.last_username.clone()),
                password: opts.auto_login_pass.clone().unwrap_or_default(),
                error: None,
            },
            settings: opts.settings,
            client,
            session: None,
            page: NavPage::Dashboard,
            status_msg: None,
            dashboard: None,
            inspections: Vec::new(),
            ncrs: Vec::new(),
            capas: Vec::new(),
            products: Vec::new(),
            lots: Vec::new(),
            search: String::new(),
            status_filter: String::new(),
            sort: SortState::default(),
            detail: DetailView::None,
            form: FormMode::None,
            insp_form: InspectionForm {
                lot_id: String::new(),
                notes: String::new(),
                error: None,
            },
            ncr_form: NcrForm {
                product_id: String::new(),
                title: String::new(),
                description: String::new(),
                severity: "minor".into(),
                quantity: "1".into(),
                error: None,
            },
            capa_form: CapaForm {
                title: String::new(),
                root_cause: String::new(),
                action_plan: String::new(),
                due_date: chrono::Utc::now().date_naive().to_string(),
                error: None,
            },
            inspection_detail: None,
            ncr_detail: None,
            capa_detail: None,
            login_promise: None,
            dash_promise: None,
            list_promise: None,
            detail_promise: None,
            mutate_promise: None,
            bootstrap_promise: None,
            loading: false,
            list_error: None,
            shot_step: if opts.screenshot_dir.is_some() {
                Some(ShotStep::WaitLogin)
            } else {
                None
            },
            screenshot_dir: opts.screenshot_dir,
            shot_frames: 0,
            pending_shot_name: None,
            should_close: false,
        }
    }

    fn logged_in(&self) -> bool {
        self.session.is_some()
    }

    fn spawn_login(&mut self) {
        let mut client = self.client.clone();
        client.set_base_url(self.settings.server_url.clone());
        let user = self.login.username.clone();
        let pass = self.login.password.clone();
        self.loading = true;
        self.login.error = None;
        self.login_promise = Some(Promise::spawn_thread("login", move || {
            client.login(&user, &pass)
        }));
    }

    fn spawn_dashboard(&mut self) {
        let client = self.authed_client();
        self.loading = true;
        self.list_error = None;
        self.dash_promise = Some(Promise::spawn_thread("dashboard", move || {
            client.dashboard()
        }));
    }

    fn spawn_list(&mut self) {
        let client = self.authed_client();
        let page = self.page;
        let status = self.status_filter.clone();
        self.loading = true;
        self.list_error = None;
        self.list_promise = Some(Promise::spawn_thread("list", move || {
            let status = if status.is_empty() {
                None
            } else {
                Some(status.as_str())
            };
            match page {
                NavPage::Inspections => {
                    let (rows, _) = client.list_inspections(status, 200, 0)?;
                    Ok(ListPayload::Inspections(rows))
                }
                NavPage::Nonconformances => {
                    let (rows, _) = client.list_ncrs(status, 200, 0)?;
                    Ok(ListPayload::Ncrs(rows))
                }
                NavPage::CorrectiveActions => {
                    let (rows, _) = client.list_capas(status, 200, 0)?;
                    Ok(ListPayload::Capas(rows))
                }
                _ => Ok(ListPayload::Inspections(Vec::new())),
            }
        }));
    }

    fn spawn_bootstrap(&mut self) {
        let client = self.authed_client();
        self.bootstrap_promise = Some(Promise::spawn_thread("bootstrap", move || {
            let (products, _) = client.list_products(None)?;
            let (lots, _) = client.list_lots(None)?;
            Ok(BootstrapPayload { products, lots })
        }));
    }

    fn spawn_detail(&mut self, detail: DetailView) {
        let client = self.authed_client();
        self.loading = true;
        self.detail_promise = Some(Promise::spawn_thread("detail", move || match detail {
            DetailView::Inspection(id) => Ok(DetailPayload::Inspection(client.get_inspection(id)?)),
            DetailView::Ncr(id) => Ok(DetailPayload::Ncr(client.get_ncr(id)?)),
            DetailView::Capa(id) => Ok(DetailPayload::Capa(client.get_capa(id)?)),
            DetailView::None => Err(ApiError::Invalid("no detail".into())),
        }));
    }

    fn authed_client(&self) -> ApiClient {
        let mut c = self.client.clone();
        c.set_base_url(self.settings.server_url.clone());
        if let Some(s) = &self.session {
            c.set_access_token(Some(s.access_token.clone()));
        }
        c
    }

    fn poll_promises(&mut self, ctx: &egui::Context) {
        if self
            .login_promise
            .as_ref()
            .is_some_and(|p| p.ready().is_some())
        {
            if let Some(p) = self.login_promise.take() {
                self.loading = false;
                match p.block_and_take() {
                    Ok(session) => {
                        self.client
                            .set_access_token(Some(session.access_token.clone()));
                        self.settings.last_username = session.user.username.clone();
                        let _ = self.settings.save();
                        self.session = Some(session);
                        self.login.password.clear();
                        self.login.error = None;
                        self.page = NavPage::Dashboard;
                        self.spawn_dashboard();
                        self.spawn_bootstrap();
                    }
                    Err(e) => {
                        self.login.error = Some(e.message());
                    }
                }
                ctx.request_repaint();
            }
        }

        if self
            .dash_promise
            .as_ref()
            .is_some_and(|p| p.ready().is_some())
        {
            if let Some(p) = self.dash_promise.take() {
                self.loading = false;
                match p.block_and_take() {
                    Ok(d) => {
                        self.dashboard = Some(d);
                        self.list_error = None;
                    }
                    Err(e) => self.list_error = Some(e.message()),
                }
                ctx.request_repaint();
            }
        }

        if self
            .list_promise
            .as_ref()
            .is_some_and(|p| p.ready().is_some())
        {
            if let Some(p) = self.list_promise.take() {
                self.loading = false;
                match p.block_and_take() {
                    Ok(ListPayload::Inspections(rows)) => {
                        self.inspections = rows;
                        self.list_error = None;
                    }
                    Ok(ListPayload::Ncrs(rows)) => {
                        self.ncrs = rows;
                        self.list_error = None;
                    }
                    Ok(ListPayload::Capas(rows)) => {
                        self.capas = rows;
                        self.list_error = None;
                    }
                    Err(e) => self.list_error = Some(e.message()),
                }
                ctx.request_repaint();
            }
        }

        if self
            .detail_promise
            .as_ref()
            .is_some_and(|p| p.ready().is_some())
        {
            if let Some(p) = self.detail_promise.take() {
                self.loading = false;
                match p.block_and_take() {
                    Ok(DetailPayload::Inspection(d)) => self.inspection_detail = Some(d),
                    Ok(DetailPayload::Ncr(d)) => self.ncr_detail = Some(d),
                    Ok(DetailPayload::Capa(d)) => self.capa_detail = Some(d),
                    Err(e) => self.list_error = Some(e.message()),
                }
                ctx.request_repaint();
            }
        }

        if self
            .mutate_promise
            .as_ref()
            .is_some_and(|p| p.ready().is_some())
        {
            if let Some(p) = self.mutate_promise.take() {
                self.loading = false;
                match p.block_and_take() {
                    Ok(MutatePayload::Ok(msg)) => {
                        self.status_msg = Some(msg);
                        self.form = FormMode::None;
                        self.refresh_current();
                    }
                    Ok(MutatePayload::InspectionCreated(_))
                    | Ok(MutatePayload::NcrSaved(_))
                    | Ok(MutatePayload::CapaSaved(_)) => {
                        self.status_msg = Some("Saved.".into());
                        self.form = FormMode::None;
                        self.detail = DetailView::None;
                        self.refresh_current();
                    }
                    Err(e) => {
                        let msg = e.message();
                        match &mut self.form {
                            FormMode::CreateInspection => self.insp_form.error = Some(msg),
                            FormMode::CreateNcr | FormMode::EditNcr(_) => {
                                self.ncr_form.error = Some(msg)
                            }
                            FormMode::CreateCapa | FormMode::EditCapa(_) => {
                                self.capa_form.error = Some(msg)
                            }
                            FormMode::None => self.list_error = Some(msg),
                        }
                    }
                }
                ctx.request_repaint();
            }
        }

        if self
            .bootstrap_promise
            .as_ref()
            .is_some_and(|p| p.ready().is_some())
        {
            if let Some(p) = self.bootstrap_promise.take() {
                if let Ok(b) = p.block_and_take() {
                    self.products = b.products;
                    self.lots = b.lots;
                }
            }
        }
    }

    fn refresh_current(&mut self) {
        match self.page {
            NavPage::Dashboard => self.spawn_dashboard(),
            NavPage::Inspections | NavPage::Nonconformances | NavPage::CorrectiveActions => {
                self.spawn_list()
            }
            NavPage::Settings => {}
        }
    }

    fn handle_shortcuts(&mut self, ctx: &egui::Context) {
        let input = ctx.input(|i| {
            (
                i.modifiers.command || i.modifiers.ctrl,
                i.key_pressed(egui::Key::R),
                i.key_pressed(egui::Key::N),
                i.key_pressed(egui::Key::Comma),
                i.key_pressed(egui::Key::Escape),
                i.key_pressed(egui::Key::Num1),
                i.key_pressed(egui::Key::Num2),
                i.key_pressed(egui::Key::Num3),
                i.key_pressed(egui::Key::Num4),
                i.key_pressed(egui::Key::Num5),
                i.key_pressed(egui::Key::F),
            )
        });
        let (mod_key, r, n, comma, esc, k1, k2, k3, k4, k5, f) = input;
        if !self.logged_in() {
            return;
        }
        if esc {
            self.detail = DetailView::None;
            self.form = FormMode::None;
        }
        if mod_key && r {
            self.refresh_current();
        }
        if mod_key && comma {
            self.page = NavPage::Settings;
        }
        if mod_key && n {
            match self.page {
                NavPage::Inspections => self.form = FormMode::CreateInspection,
                NavPage::Nonconformances => self.form = FormMode::CreateNcr,
                NavPage::CorrectiveActions => self.form = FormMode::CreateCapa,
                _ => {}
            }
        }
        if mod_key && f {
            self.status_msg = Some("Search field: type to filter the table.".into());
        }
        if mod_key && k1 {
            self.goto(NavPage::Dashboard);
        }
        if mod_key && k2 {
            self.goto(NavPage::Inspections);
        }
        if mod_key && k3 {
            self.goto(NavPage::Nonconformances);
        }
        if mod_key && k4 {
            self.goto(NavPage::CorrectiveActions);
        }
        if mod_key && k5 {
            self.goto(NavPage::Settings);
        }
    }

    fn goto(&mut self, page: NavPage) {
        if self.page != page {
            self.page = page;
            self.detail = DetailView::None;
            self.form = FormMode::None;
            self.search.clear();
            self.status_filter.clear();
            self.refresh_current();
        }
    }

    fn filtered_inspections(&self) -> Vec<&Inspection> {
        let q = self.search.to_lowercase();
        let mut rows: Vec<&Inspection> = self
            .inspections
            .iter()
            .filter(|r| {
                q.is_empty()
                    || r.inspection_number.to_lowercase().contains(&q)
                    || r.status.to_lowercase().contains(&q)
                    || r.result.to_lowercase().contains(&q)
                    || r.notes.to_lowercase().contains(&q)
            })
            .collect();
        if let Some(key) = self.sort.key {
            rows.sort_by(|a, b| {
                let ord = match key {
                    SortKey::Number => a.inspection_number.cmp(&b.inspection_number),
                    SortKey::Status => a.status.cmp(&b.status),
                    SortKey::Result => a.result.cmp(&b.result),
                    SortKey::Opened => a.started_at.cmp(&b.started_at),
                    _ => std::cmp::Ordering::Equal,
                };
                if self.sort.asc {
                    ord
                } else {
                    ord.reverse()
                }
            });
        }
        rows
    }

    fn filtered_ncrs(&self) -> Vec<&Nonconformance> {
        let q = self.search.to_lowercase();
        let mut rows: Vec<&Nonconformance> = self
            .ncrs
            .iter()
            .filter(|r| {
                q.is_empty()
                    || r.ncr_number.to_lowercase().contains(&q)
                    || r.title.to_lowercase().contains(&q)
                    || r.status.to_lowercase().contains(&q)
                    || r.severity.to_lowercase().contains(&q)
            })
            .collect();
        if let Some(key) = self.sort.key {
            rows.sort_by(|a, b| {
                let ord = match key {
                    SortKey::Number => a.ncr_number.cmp(&b.ncr_number),
                    SortKey::Status => a.status.cmp(&b.status),
                    SortKey::Severity => a.severity.cmp(&b.severity),
                    SortKey::Title => a.title.cmp(&b.title),
                    SortKey::Opened => a.opened_at.cmp(&b.opened_at),
                    _ => std::cmp::Ordering::Equal,
                };
                if self.sort.asc {
                    ord
                } else {
                    ord.reverse()
                }
            });
        }
        rows
    }

    fn filtered_capas(&self) -> Vec<&CorrectiveAction> {
        let q = self.search.to_lowercase();
        let mut rows: Vec<&CorrectiveAction> = self
            .capas
            .iter()
            .filter(|r| {
                q.is_empty()
                    || r.ca_number.to_lowercase().contains(&q)
                    || r.title.to_lowercase().contains(&q)
                    || r.status.to_lowercase().contains(&q)
            })
            .collect();
        if let Some(key) = self.sort.key {
            rows.sort_by(|a, b| {
                let ord = match key {
                    SortKey::Number => a.ca_number.cmp(&b.ca_number),
                    SortKey::Status => a.status.cmp(&b.status),
                    SortKey::Title => a.title.cmp(&b.title),
                    SortKey::Due => a.due_date.cmp(&b.due_date),
                    SortKey::Opened => a.opened_at.cmp(&b.opened_at),
                    _ => std::cmp::Ordering::Equal,
                };
                if self.sort.asc {
                    ord
                } else {
                    ord.reverse()
                }
            });
        }
        rows
    }

    fn toggle_sort(&mut self, key: SortKey) {
        if self.sort.key == Some(key) {
            self.sort.asc = !self.sort.asc;
        } else {
            self.sort.key = Some(key);
            self.sort.asc = true;
        }
    }

    fn export_current(&mut self) {
        let suggested = match self.page {
            NavPage::Inspections => "inspections.csv",
            NavPage::Nonconformances => "nonconformances.csv",
            NavPage::CorrectiveActions => "corrective_actions.csv",
            _ => return,
        };
        if let Some(path) = rfd::FileDialog::new()
            .set_file_name(suggested)
            .add_filter("CSV", &["csv"])
            .save_file()
        {
            let result = match self.page {
                NavPage::Inspections => csv_export::export_inspections(&path, &self.inspections),
                NavPage::Nonconformances => csv_export::export_ncrs(&path, &self.ncrs),
                NavPage::CorrectiveActions => csv_export::export_capas(&path, &self.capas),
                _ => Ok(()),
            };
            match result {
                Ok(()) => self.status_msg = Some(format!("Exported {}", path.display())),
                Err(e) => self.list_error = Some(e),
            }
        }
    }

    fn submit_inspection(&mut self) {
        let lot_id = match Uuid::parse_str(self.insp_form.lot_id.trim()) {
            Ok(id) => id,
            Err(_) => {
                self.insp_form.error = Some("lot_id must be a valid UUID.".into());
                return;
            }
        };
        let body = NewInspection {
            lot_id,
            plan_id: None,
            station_id: None,
            inspector_id: None,
            sample_size: None,
            notes: self.insp_form.notes.clone(),
        };
        let client = self.authed_client();
        self.loading = true;
        self.insp_form.error = None;
        self.mutate_promise = Some(Promise::spawn_thread("create-insp", move || {
            let d = client.create_inspection(&body)?;
            Ok(MutatePayload::InspectionCreated(d))
        }));
    }

    fn submit_ncr(&mut self) {
        let product_id = match Uuid::parse_str(self.ncr_form.product_id.trim()) {
            Ok(id) => id,
            Err(_) => {
                self.ncr_form.error = Some("product_id must be a valid UUID.".into());
                return;
            }
        };
        if self.ncr_form.title.trim().is_empty() {
            self.ncr_form.error = Some("Title is required.".into());
            return;
        }
        let qty: f64 = self.ncr_form.quantity.trim().parse().unwrap_or(-1.0);
        if qty < 0.0 {
            self.ncr_form.error = Some("Quantity must be a non-negative number.".into());
            return;
        }
        let client = self.authed_client();
        self.loading = true;
        self.ncr_form.error = None;
        match self.form {
            FormMode::CreateNcr => {
                let body = NewNonconformance {
                    inspection_id: None,
                    lot_id: None,
                    product_id,
                    title: self.ncr_form.title.clone(),
                    description: self.ncr_form.description.clone(),
                    severity: self.ncr_form.severity.clone(),
                    quantity_affected: qty,
                    assigned_to: None,
                };
                self.mutate_promise = Some(Promise::spawn_thread("create-ncr", move || {
                    let d = client.create_ncr(&body)?;
                    Ok(MutatePayload::NcrSaved(d))
                }));
            }
            FormMode::EditNcr(id) => {
                let body = crate::api::PatchNonconformance {
                    title: Some(self.ncr_form.title.clone()),
                    description: Some(self.ncr_form.description.clone()),
                    severity: Some(self.ncr_form.severity.clone()),
                    quantity_affected: Some(qty),
                };
                self.mutate_promise = Some(Promise::spawn_thread("edit-ncr", move || {
                    let d = client.update_ncr(id, &body)?;
                    Ok(MutatePayload::NcrSaved(d))
                }));
            }
            _ => {}
        }
    }

    fn submit_capa(&mut self) {
        if self.capa_form.title.trim().is_empty() {
            self.capa_form.error = Some("Title is required.".into());
            return;
        }
        if self.capa_form.action_plan.trim().is_empty() {
            self.capa_form.error = Some("Action plan is required.".into());
            return;
        }
        let client = self.authed_client();
        self.loading = true;
        self.capa_form.error = None;
        match self.form {
            FormMode::CreateCapa => {
                let body = NewCorrectiveAction {
                    nonconformance_id: None,
                    title: self.capa_form.title.clone(),
                    root_cause: self.capa_form.root_cause.clone(),
                    action_plan: self.capa_form.action_plan.clone(),
                    owner_id: None,
                    due_date: self.capa_form.due_date.clone(),
                };
                self.mutate_promise = Some(Promise::spawn_thread("create-capa", move || {
                    let d = client.create_capa(&body)?;
                    Ok(MutatePayload::CapaSaved(d))
                }));
            }
            FormMode::EditCapa(id) => {
                let body = crate::api::PatchCorrectiveAction {
                    title: Some(self.capa_form.title.clone()),
                    root_cause: Some(self.capa_form.root_cause.clone()),
                    action_plan: Some(self.capa_form.action_plan.clone()),
                    due_date: Some(self.capa_form.due_date.clone()),
                };
                self.mutate_promise = Some(Promise::spawn_thread("edit-capa", move || {
                    let d = client.update_capa(id, &body)?;
                    Ok(MutatePayload::CapaSaved(d))
                }));
            }
            _ => {}
        }
    }

    fn request_screenshot(&mut self, ctx: &egui::Context, name: &str) {
        self.pending_shot_name = Some(name.to_string());
        ctx.send_viewport_cmd(egui::ViewportCommand::Screenshot);
    }

    fn drain_screenshots(&mut self, ctx: &egui::Context) {
        let Some(name) = self.pending_shot_name.clone() else {
            return;
        };
        let image = ctx.input(|i| {
            i.events.iter().find_map(|e| {
                if let egui::Event::Screenshot { image, .. } = e {
                    Some(image.clone())
                } else {
                    None
                }
            })
        });
        if let Some(color_image) = image {
            if let Some(dir) = &self.screenshot_dir {
                let _ = std::fs::create_dir_all(dir);
                let path = dir.join(format!("{name}.png"));
                let w = color_image.width() as u32;
                let h = color_image.height() as u32;
                let mut rgba = Vec::with_capacity(color_image.pixels.len() * 4);
                for p in &color_image.pixels {
                    rgba.push(p.r());
                    rgba.push(p.g());
                    rgba.push(p.b());
                    rgba.push(p.a());
                }
                if let Some(img) = image::RgbaImage::from_raw(w, h, rgba) {
                    let _ = img.save(&path);
                }
            }
            self.pending_shot_name = None;
        }
    }

    fn advance_screenshots(&mut self, ctx: &egui::Context) {
        let Some(step) = self.shot_step else {
            return;
        };
        self.shot_frames += 1;
        match step {
            ShotStep::WaitLogin => {
                if self.shot_frames > 8 {
                    self.shot_step = Some(ShotStep::CaptureLogin);
                    self.shot_frames = 0;
                }
            }
            ShotStep::CaptureLogin => {
                self.request_screenshot(ctx, "01_login");
                self.shot_step = Some(ShotStep::WaitDashboard);
                self.shot_frames = 0;
                if !self.login.username.is_empty() && !self.login.password.is_empty() {
                    self.spawn_login();
                }
            }
            ShotStep::WaitDashboard => {
                if self.logged_in() && self.dashboard.is_some() && self.shot_frames > 12 {
                    self.shot_step = Some(ShotStep::CaptureDashboard);
                    self.shot_frames = 0;
                } else if self.shot_frames > 180 {
                    self.shot_step = Some(ShotStep::CaptureDashboard);
                    self.shot_frames = 0;
                }
            }
            ShotStep::CaptureDashboard => {
                self.page = NavPage::Dashboard;
                self.request_screenshot(ctx, "02_dashboard");
                self.shot_step = Some(ShotStep::CaptureInspections);
                self.shot_frames = 0;
                self.goto(NavPage::Inspections);
            }
            ShotStep::CaptureInspections => {
                if self.shot_frames > 20 {
                    self.request_screenshot(ctx, "03_inspections");
                    self.shot_step = Some(ShotStep::CaptureNcrs);
                    self.shot_frames = 0;
                    self.goto(NavPage::Nonconformances);
                }
            }
            ShotStep::CaptureNcrs => {
                if self.shot_frames > 20 {
                    self.request_screenshot(ctx, "04_nonconformances");
                    self.shot_step = Some(ShotStep::CaptureCapas);
                    self.shot_frames = 0;
                    self.goto(NavPage::CorrectiveActions);
                }
            }
            ShotStep::CaptureCapas => {
                if self.shot_frames > 20 {
                    self.request_screenshot(ctx, "05_corrective_actions");
                    self.shot_step = Some(ShotStep::CaptureSettings);
                    self.shot_frames = 0;
                    self.goto(NavPage::Settings);
                }
            }
            ShotStep::CaptureSettings => {
                if self.shot_frames > 12 {
                    self.request_screenshot(ctx, "06_settings");
                    self.shot_step = Some(ShotStep::Done);
                    self.shot_frames = 0;
                }
            }
            ShotStep::Done => {
                if self.shot_frames > 15 && self.pending_shot_name.is_none() {
                    self.should_close = true;
                }
            }
        }
        ctx.request_repaint();
    }
}

impl eframe::App for QcApp {
    fn update(&mut self, ctx: &egui::Context, _frame: &mut eframe::Frame) {
        apply_theme(ctx, self.settings.theme);
        self.poll_promises(ctx);
        self.handle_shortcuts(ctx);
        self.drain_screenshots(ctx);
        self.advance_screenshots(ctx);

        if self.should_close {
            ctx.send_viewport_cmd(egui::ViewportCommand::Close);
            return;
        }

        if !self.logged_in() {
            self.ui_login(ctx);
            return;
        }

        egui::SidePanel::left("sidebar")
            .resizable(true)
            .default_width(200.0)
            .show(ctx, |ui| {
                ui.add_space(12.0);
                ui.heading("Quality Control");
                if let Some(s) = &self.session {
                    ui.label(
                        egui::RichText::new(format!("{} · {}", s.user.full_name, s.user.role))
                            .small()
                            .weak(),
                    );
                }
                ui.add_space(8.0);
                ui.separator();
                ui.add_space(4.0);
                for page in NavPage::all() {
                    let selected = self.page == *page;
                    if ui
                        .selectable_label(selected, page.label())
                        .on_hover_text(match page {
                            NavPage::Dashboard => "Ctrl/Cmd+1",
                            NavPage::Inspections => "Ctrl/Cmd+2 · Ctrl/Cmd+N new",
                            NavPage::Nonconformances => "Ctrl/Cmd+3",
                            NavPage::CorrectiveActions => "Ctrl/Cmd+4",
                            NavPage::Settings => "Ctrl/Cmd+5 or Ctrl/Cmd+,",
                        })
                        .clicked()
                    {
                        self.goto(*page);
                    }
                }
                ui.with_layout(egui::Layout::bottom_up(egui::Align::LEFT), |ui| {
                    ui.add_space(8.0);
                    if ui.button("Sign out").clicked() {
                        self.session = None;
                        self.client.set_access_token(None);
                        self.dashboard = None;
                    }
                    ui.label(
                        egui::RichText::new("Ctrl/Cmd+R refresh")
                            .small()
                            .weak(),
                    );
                });
            });

        egui::TopBottomPanel::top("topbar").show(ctx, |ui| {
            ui.horizontal(|ui| {
                ui.heading(self.page.label());
                ui.with_layout(egui::Layout::right_to_left(egui::Align::Center), |ui| {
                    if self.loading {
                        loading_spinner(ui);
                    }
                    if matches!(
                        self.page,
                        NavPage::Inspections
                            | NavPage::Nonconformances
                            | NavPage::CorrectiveActions
                    ) {
                        if ui.button("Export CSV").clicked() {
                            self.export_current();
                        }
                        if ui.button("New").clicked() {
                            match self.page {
                                NavPage::Inspections => self.form = FormMode::CreateInspection,
                                NavPage::Nonconformances => self.form = FormMode::CreateNcr,
                                NavPage::CorrectiveActions => self.form = FormMode::CreateCapa,
                                _ => {}
                            }
                        }
                        if ui.button("Refresh").clicked() {
                            self.refresh_current();
                        }
                    } else if self.page == NavPage::Dashboard && ui.button("Refresh").clicked() {
                        self.refresh_current();
                    }
                });
            });
            if let Some(msg) = &self.status_msg {
                ui.label(egui::RichText::new(msg).color(egui::Color32::from_rgb(40, 140, 90)));
            }
            if let Some(err) = &self.list_error {
                error_banner(ui, err);
            }
        });

        egui::CentralPanel::default().show(ctx, |ui| match self.page {
            NavPage::Dashboard => self.ui_dashboard(ui),
            NavPage::Inspections => self.ui_inspections(ui),
            NavPage::Nonconformances => self.ui_ncrs(ui),
            NavPage::CorrectiveActions => self.ui_capas(ui),
            NavPage::Settings => self.ui_settings(ui),
        });
    }
}

impl QcApp {
    fn ui_login(&mut self, ctx: &egui::Context) {
        egui::CentralPanel::default().show(ctx, |ui| {
            ui.vertical_centered(|ui| {
                ui.add_space(80.0);
                ui.heading("Quality Control");
                ui.label("Sign in to the plant API");
                ui.add_space(16.0);
                egui::Frame::group(ui.style()).show(ui, |ui| {
                    ui.set_width(360.0);
                    ui.label("Server URL");
                    ui.text_edit_singleline(&mut self.settings.server_url);
                    ui.add_space(6.0);
                    ui.label("Username");
                    ui.text_edit_singleline(&mut self.login.username);
                    ui.label("Password");
                    ui.add(egui::TextEdit::singleline(&mut self.login.password).password(true));
                    ui.add_space(8.0);
                    let enter = ui.input(|i| i.key_pressed(egui::Key::Enter));
                    let clicked = ui
                        .add_enabled(!self.loading, egui::Button::new("Sign in"))
                        .clicked();
                    if (clicked || enter)
                        && !self.login.username.is_empty()
                        && !self.login.password.is_empty()
                    {
                        self.client.set_base_url(self.settings.server_url.clone());
                        let _ = self.settings.save();
                        self.spawn_login();
                    }
                    if self.loading {
                        loading_spinner(ui);
                    }
                    if let Some(err) = &self.login.error {
                        ui.colored_label(egui::Color32::from_rgb(200, 60, 60), err);
                    }
                });
            });
        });
    }

    fn ui_dashboard(&mut self, ui: &mut egui::Ui) {
        if self.loading && self.dashboard.is_none() {
            loading_spinner(ui);
            return;
        }
        let Some(d) = self.dashboard.clone() else {
            empty_state(ui, "No dashboard data yet.", "Refresh to load plant stats.");
            return;
        };
        ui.add_space(4.0);
        ui.label("Key quality-control metrics for the plant.");
        ui.add_space(12.0);
        egui::Grid::new("dash_grid")
            .num_columns(3)
            .spacing([16.0, 16.0])
            .show(ui, |ui| {
                metric(ui, "Open inspections", d.open_inspections);
                metric(ui, "Lots on hold", d.lots_on_hold);
                metric(ui, "Open NCRs", d.open_nonconformances);
                metric(ui, "Open CAPAs", d.open_corrective_actions);
                metric(ui, "Overdue CAPAs", d.overdue_corrective_actions);
                metric(ui, "Inspections (30d)", d.inspections_last_30_days);
                metric(ui, "Pass (30d)", d.pass_last_30_days);
                metric(ui, "Fail (30d)", d.fail_last_30_days);
                metric(ui, "Conditional (30d)", d.conditional_last_30_days);
            });
        ui.add_space(16.0);
        if let Some(rate) = d.pass_rate_last_30_days {
            ui.label(format!("Pass rate (30d): {:.1}%", rate * 100.0));
        } else {
            ui.label("Pass rate (30d): —");
        }
        if !d.defects_by_severity.is_empty() {
            ui.add_space(8.0);
            ui.strong("Defects by severity");
            for s in &d.defects_by_severity {
                ui.label(format!("{}: {}", s.severity, s.count));
            }
        }
    }

    fn ui_inspections(&mut self, ui: &mut egui::Ui) {
        self.ui_toolbar(ui, &["", "in_progress", "completed", "cancelled"]);
        if matches!(self.form, FormMode::CreateInspection) {
            self.ui_inspection_form(ui);
            ui.separator();
        }
        if let DetailView::Inspection(_) = self.detail {
            self.ui_inspection_detail(ui);
            return;
        }
        let rows: Vec<Inspection> = self.filtered_inspections().into_iter().cloned().collect();
        if !self.loading && rows.is_empty() {
            empty_state(
                ui,
                "No inspections match.",
                "Adjust search/filter or create a new inspection (Ctrl/Cmd+N).",
            );
            return;
        }
        egui::ScrollArea::both().show(ui, |ui| {
            egui::Grid::new("insp_table")
                .striped(true)
                .min_col_width(90.0)
                .show(ui, |ui| {
                    if ui.button("Number").clicked() {
                        self.toggle_sort(SortKey::Number);
                    }
                    if ui.button("Status").clicked() {
                        self.toggle_sort(SortKey::Status);
                    }
                    if ui.button("Result").clicked() {
                        self.toggle_sort(SortKey::Result);
                    }
                    if ui.button("Started").clicked() {
                        self.toggle_sort(SortKey::Opened);
                    }
                    ui.label("Sample");
                    ui.label("");
                    ui.end_row();
                    for r in &rows {
                        ui.label(&r.inspection_number);
                        status_badge(ui, &r.status);
                        status_badge(ui, &r.result);
                        ui.label(r.started_at.format("%Y-%m-%d %H:%M").to_string());
                        ui.label(r.sample_size.to_string());
                        if ui.button("Open").clicked() {
                            let id = r.id;
                            self.detail = DetailView::Inspection(id);
                            self.spawn_detail(DetailView::Inspection(id));
                        }
                        ui.end_row();
                    }
                });
        });
    }

    fn ui_ncrs(&mut self, ui: &mut egui::Ui) {
        self.ui_toolbar(
            ui,
            &["", "open", "under_review", "dispositioned", "closed", "void"],
        );
        if matches!(self.form, FormMode::CreateNcr | FormMode::EditNcr(_)) {
            self.ui_ncr_form(ui);
            ui.separator();
        }
        if let DetailView::Ncr(_) = self.detail {
            self.ui_ncr_detail(ui);
            return;
        }
        let rows: Vec<Nonconformance> = self.filtered_ncrs().into_iter().cloned().collect();
        if !self.loading && rows.is_empty() {
            empty_state(
                ui,
                "No nonconformances match.",
                "Create an NCR with Ctrl/Cmd+N.",
            );
            return;
        }
        egui::ScrollArea::both().show(ui, |ui| {
            egui::Grid::new("ncr_table")
                .striped(true)
                .min_col_width(90.0)
                .show(ui, |ui| {
                    if ui.button("Number").clicked() {
                        self.toggle_sort(SortKey::Number);
                    }
                    if ui.button("Title").clicked() {
                        self.toggle_sort(SortKey::Title);
                    }
                    if ui.button("Severity").clicked() {
                        self.toggle_sort(SortKey::Severity);
                    }
                    if ui.button("Status").clicked() {
                        self.toggle_sort(SortKey::Status);
                    }
                    ui.label("");
                    ui.end_row();
                    for r in &rows {
                        ui.label(&r.ncr_number);
                        ui.label(&r.title);
                        status_badge(ui, &r.severity);
                        status_badge(ui, &r.status);
                        ui.horizontal(|ui| {
                            if ui.button("Open").clicked() {
                                let id = r.id;
                                self.detail = DetailView::Ncr(id);
                                self.spawn_detail(DetailView::Ncr(id));
                            }
                            if ui.button("Edit").clicked() {
                                self.ncr_form.product_id = r.product_id.to_string();
                                self.ncr_form.title = r.title.clone();
                                self.ncr_form.description = r.description.clone();
                                self.ncr_form.severity = r.severity.clone();
                                self.ncr_form.quantity = r.quantity_affected.to_string();
                                self.form = FormMode::EditNcr(r.id);
                            }
                        });
                        ui.end_row();
                    }
                });
        });
    }

    fn ui_capas(&mut self, ui: &mut egui::Ui) {
        self.ui_toolbar(
            ui,
            &[
                "",
                "open",
                "in_progress",
                "verification",
                "closed",
                "cancelled",
            ],
        );
        if matches!(self.form, FormMode::CreateCapa | FormMode::EditCapa(_)) {
            self.ui_capa_form(ui);
            ui.separator();
        }
        if let DetailView::Capa(_) = self.detail {
            self.ui_capa_detail(ui);
            return;
        }
        let rows: Vec<CorrectiveAction> = self.filtered_capas().into_iter().cloned().collect();
        if !self.loading && rows.is_empty() {
            empty_state(
                ui,
                "No corrective actions match.",
                "Create a CAPA with Ctrl/Cmd+N.",
            );
            return;
        }
        egui::ScrollArea::both().show(ui, |ui| {
            egui::Grid::new("capa_table")
                .striped(true)
                .min_col_width(90.0)
                .show(ui, |ui| {
                    if ui.button("Number").clicked() {
                        self.toggle_sort(SortKey::Number);
                    }
                    if ui.button("Title").clicked() {
                        self.toggle_sort(SortKey::Title);
                    }
                    if ui.button("Status").clicked() {
                        self.toggle_sort(SortKey::Status);
                    }
                    if ui.button("Due").clicked() {
                        self.toggle_sort(SortKey::Due);
                    }
                    ui.label("");
                    ui.end_row();
                    for r in &rows {
                        ui.label(&r.ca_number);
                        ui.label(&r.title);
                        status_badge(ui, &r.status);
                        ui.label(&r.due_date);
                        ui.horizontal(|ui| {
                            if ui.button("Open").clicked() {
                                let id = r.id;
                                self.detail = DetailView::Capa(id);
                                self.spawn_detail(DetailView::Capa(id));
                            }
                            if ui.button("Edit").clicked() {
                                self.capa_form.title = r.title.clone();
                                self.capa_form.root_cause = r.root_cause.clone();
                                self.capa_form.action_plan = r.action_plan.clone();
                                self.capa_form.due_date = r.due_date.clone();
                                self.form = FormMode::EditCapa(r.id);
                            }
                        });
                        ui.end_row();
                    }
                });
        });
    }

    fn ui_settings(&mut self, ui: &mut egui::Ui) {
        ui.label("Server connection and appearance. Changes are saved on disk.");
        ui.add_space(8.0);
        ui.label("Server URL");
        ui.text_edit_singleline(&mut self.settings.server_url);
        ui.add_space(8.0);
        ui.label("Theme");
        ui.horizontal(|ui| {
            ui.radio_value(&mut self.settings.theme, ThemePreference::Light, "Light");
            ui.radio_value(&mut self.settings.theme, ThemePreference::Dark, "Dark");
            ui.radio_value(&mut self.settings.theme, ThemePreference::System, "System");
        });
        ui.add_space(8.0);
        if ui.button("Save settings").clicked() {
            self.client.set_base_url(self.settings.server_url.clone());
            match self.settings.save() {
                Ok(()) => {
                    self.status_msg = Some(format!("Saved to {}", AppSettings::path_display()))
                }
                Err(e) => self.list_error = Some(e),
            }
        }
        ui.add_space(12.0);
        ui.label(format!("Settings file: {}", AppSettings::path_display()));
        if let Some(s) = &self.session {
            ui.label(format!(
                "Signed in as {} ({})",
                s.user.username, s.user.role
            ));
        }
        ui.add_space(12.0);
        ui.collapsing("Keyboard shortcuts", |ui| {
            ui.label("Ctrl/Cmd+1…5 — navigate sections");
            ui.label("Ctrl/Cmd+R — refresh");
            ui.label("Ctrl/Cmd+N — new record on list pages");
            ui.label("Ctrl/Cmd+, — settings");
            ui.label("Esc — close detail/form");
            ui.label("Enter — submit login");
        });
    }

    fn ui_toolbar(&mut self, ui: &mut egui::Ui, statuses: &[&str]) {
        ui.horizontal(|ui| {
            searchable_field(ui, &mut self.search);
            ui.label("Status");
            egui::ComboBox::from_id_salt("status_filter")
                .selected_text(if self.status_filter.is_empty() {
                    "all"
                } else {
                    &self.status_filter
                })
                .show_ui(ui, |ui| {
                    for s in statuses {
                        let label = if s.is_empty() { "all" } else { s };
                        if ui
                            .selectable_label(self.status_filter == *s, label)
                            .clicked()
                        {
                            self.status_filter = (*s).to_string();
                            self.spawn_list();
                        }
                    }
                });
        });
        ui.add_space(6.0);
    }

    fn ui_inspection_form(&mut self, ui: &mut egui::Ui) {
        ui.group(|ui| {
            ui.strong("New inspection");
            ui.label("Lot ID (UUID)");
            if !self.lots.is_empty() {
                egui::ComboBox::from_id_salt("lot_pick")
                    .selected_text(if self.insp_form.lot_id.is_empty() {
                        "Select lot".to_string()
                    } else {
                        self.lots
                            .iter()
                            .find(|l| l.id.to_string() == self.insp_form.lot_id)
                            .map(|l| l.lot_number.clone())
                            .unwrap_or_else(|| self.insp_form.lot_id.clone())
                    })
                    .show_ui(ui, |ui| {
                        for lot in &self.lots {
                            if ui
                                .selectable_label(
                                    self.insp_form.lot_id == lot.id.to_string(),
                                    format!("{} ({})", lot.lot_number, lot.status),
                                )
                                .clicked()
                            {
                                self.insp_form.lot_id = lot.id.to_string();
                            }
                        }
                    });
            }
            ui.text_edit_singleline(&mut self.insp_form.lot_id);
            ui.label("Notes");
            ui.text_edit_multiline(&mut self.insp_form.notes);
            ui.horizontal(|ui| {
                if ui.button("Create").clicked() {
                    self.submit_inspection();
                }
                if ui.button("Cancel").clicked() {
                    self.form = FormMode::None;
                }
            });
            if let Some(err) = &self.insp_form.error {
                ui.colored_label(egui::Color32::from_rgb(200, 60, 60), err);
            }
        });
    }

    fn ui_ncr_form(&mut self, ui: &mut egui::Ui) {
        ui.group(|ui| {
            ui.strong(if matches!(self.form, FormMode::EditNcr(_)) {
                "Edit nonconformance"
            } else {
                "New nonconformance"
            });
            ui.label("Product ID (UUID)");
            if !self.products.is_empty() {
                egui::ComboBox::from_id_salt("product_pick")
                    .selected_text(if self.ncr_form.product_id.is_empty() {
                        "Select product".to_string()
                    } else {
                        self.products
                            .iter()
                            .find(|p| p.id.to_string() == self.ncr_form.product_id)
                            .map(|p| format!("{} — {}", p.part_number, p.name))
                            .unwrap_or_else(|| self.ncr_form.product_id.clone())
                    })
                    .show_ui(ui, |ui| {
                        for p in &self.products {
                            if ui
                                .selectable_label(
                                    self.ncr_form.product_id == p.id.to_string(),
                                    format!("{} — {}", p.part_number, p.name),
                                )
                                .clicked()
                            {
                                self.ncr_form.product_id = p.id.to_string();
                            }
                        }
                    });
            }
            ui.text_edit_singleline(&mut self.ncr_form.product_id);
            ui.label("Title");
            ui.text_edit_singleline(&mut self.ncr_form.title);
            ui.label("Description");
            ui.text_edit_multiline(&mut self.ncr_form.description);
            ui.horizontal(|ui| {
                ui.label("Severity");
                for s in ["minor", "major", "critical"] {
                    ui.radio_value(&mut self.ncr_form.severity, s.to_string(), s);
                }
            });
            ui.label("Quantity affected");
            ui.text_edit_singleline(&mut self.ncr_form.quantity);
            ui.horizontal(|ui| {
                if ui.button("Save").clicked() {
                    self.submit_ncr();
                }
                if ui.button("Cancel").clicked() {
                    self.form = FormMode::None;
                }
            });
            if let Some(err) = &self.ncr_form.error {
                ui.colored_label(egui::Color32::from_rgb(200, 60, 60), err);
            }
        });
    }

    fn ui_capa_form(&mut self, ui: &mut egui::Ui) {
        ui.group(|ui| {
            ui.strong(if matches!(self.form, FormMode::EditCapa(_)) {
                "Edit corrective action"
            } else {
                "New corrective action"
            });
            ui.label("Title");
            ui.text_edit_singleline(&mut self.capa_form.title);
            ui.label("Root cause");
            ui.text_edit_multiline(&mut self.capa_form.root_cause);
            ui.label("Action plan");
            ui.text_edit_multiline(&mut self.capa_form.action_plan);
            ui.label("Due date (YYYY-MM-DD)");
            ui.text_edit_singleline(&mut self.capa_form.due_date);
            ui.horizontal(|ui| {
                if ui.button("Save").clicked() {
                    self.submit_capa();
                }
                if ui.button("Cancel").clicked() {
                    self.form = FormMode::None;
                }
            });
            if let Some(err) = &self.capa_form.error {
                ui.colored_label(egui::Color32::from_rgb(200, 60, 60), err);
            }
        });
    }

    fn ui_inspection_detail(&mut self, ui: &mut egui::Ui) {
        if ui.button("← Back").clicked() {
            self.detail = DetailView::None;
            self.inspection_detail = None;
            return;
        }
        if self.loading && self.inspection_detail.is_none() {
            loading_spinner(ui);
            return;
        }
        let Some(d) = &self.inspection_detail else {
            empty_state(ui, "Inspection not loaded.", "Go back and try again.");
            return;
        };
        ui.heading(&d.inspection.inspection_number);
        ui.label(format!(
            "Status: {} · Result: {}",
            d.inspection.status, d.inspection.result
        ));
        ui.label(format!("Lot: {}", d.inspection.lot_id));
        ui.label(format!("Notes: {}", d.inspection.notes));
        ui.separator();
        ui.strong(format!("Results ({})", d.results.len()));
        for r in &d.results {
            ui.label(format!(
                "{} [{}] in_spec={} value={:?}",
                r.characteristic_code, r.sample_index, r.in_spec, r.measured_value
            ));
        }
        ui.strong(format!("Defects ({})", d.defects.len()));
        for def in &d.defects {
            ui.label(format!(
                "{} {} qty={}",
                def.code, def.severity, def.quantity
            ));
        }
        if !d.missing_characteristics.is_empty() {
            ui.colored_label(
                egui::Color32::from_rgb(180, 120, 40),
                format!("Missing: {}", d.missing_characteristics.join(", ")),
            );
        }
    }

    fn ui_ncr_detail(&mut self, ui: &mut egui::Ui) {
        if ui.button("← Back").clicked() {
            self.detail = DetailView::None;
            self.ncr_detail = None;
            return;
        }
        if self.loading && self.ncr_detail.is_none() {
            loading_spinner(ui);
            return;
        }
        let Some(d) = &self.ncr_detail else {
            empty_state(ui, "NCR not loaded.", "Go back and try again.");
            return;
        };
        ui.heading(&d.ncr.ncr_number);
        ui.label(&d.ncr.title);
        ui.label(format!(
            "{} · {} · disposition {}",
            d.ncr.severity, d.ncr.status, d.ncr.disposition
        ));
        ui.label(&d.ncr.description);
        ui.separator();
        ui.strong(format!("Linked CAPAs ({})", d.corrective_actions.len()));
        for c in &d.corrective_actions {
            ui.label(format!("{} — {} ({})", c.ca_number, c.title, c.status));
        }
    }

    fn ui_capa_detail(&mut self, ui: &mut egui::Ui) {
        if ui.button("← Back").clicked() {
            self.detail = DetailView::None;
            self.capa_detail = None;
            return;
        }
        if self.loading && self.capa_detail.is_none() {
            loading_spinner(ui);
            return;
        }
        let Some(d) = &self.capa_detail else {
            empty_state(ui, "CAPA not loaded.", "Go back and try again.");
            return;
        };
        ui.heading(&d.ca_number);
        ui.label(&d.title);
        ui.label(format!("Status: {} · Due: {}", d.status, d.due_date));
        ui.label(format!("Root cause: {}", d.root_cause));
        ui.label(format!("Action plan: {}", d.action_plan));
        if !d.verification_notes.is_empty() {
            ui.label(format!("Verification: {}", d.verification_notes));
        }
    }
}

fn metric(ui: &mut egui::Ui, label: &str, value: i32) {
    ui.group(|ui| {
        ui.set_min_width(160.0);
        ui.label(egui::RichText::new(label).weak());
        ui.heading(value.to_string());
    });
}
