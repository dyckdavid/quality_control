mod bench;
mod catalog;
mod chrome;
mod editor;
mod records;
mod theme;
mod widgets;

use crate::api::{
    ApiClient, ApiError, CompletedInspection, CorrectiveAction, Dashboard, DefectCode, Gauge,
    Inspection, InspectionDetail, Lot, NcrDetail, NewCorrectiveAction, NewInspection,
    NewNonconformance, Nonconformance, Product, Site, TokenResponse,
};
use crate::csv_export;
use crate::settings::{AppSettings, ThemePreference};
use eframe::egui;
use poll_promise::Promise;
use records::{ActionDraft, InspectionBundle, Workbench};
use std::path::PathBuf;
use std::time::{Duration, Instant};
use uuid::Uuid;

use chrome::Btn;
use theme::apply_theme;
use widgets::{empty_state, error_banner, loading_spinner, quiet_button, searchable_field, status_badge};

#[derive(Debug, Clone, Copy, PartialEq, Eq)]
pub enum NavPage {
    Dashboard,
    Lots,
    Inspections,
    Nonconformances,
    CorrectiveActions,
    Measurements,
    Products,
    Plans,
    Sites,
    Stations,
    Suppliers,
    Gauges,
    Defects,
    Checklists,
    Users,
    Audit,
    Webhooks,
    Reports,
    Search,
    Stats,
    Settings,
}

impl NavPage {
    fn label(self) -> &'static str {
        match self {
            NavPage::Dashboard => "Shop",
            NavPage::Lots => "Lots",
            NavPage::Inspections => "Inspections",
            NavPage::Nonconformances => "Alerts",
            NavPage::CorrectiveActions => "Actions",
            NavPage::Measurements => "Measurements",
            NavPage::Products => "Products",
            NavPage::Plans => "Plans",
            NavPage::Sites => "Sites",
            NavPage::Stations => "Stations",
            NavPage::Suppliers => "Suppliers",
            NavPage::Gauges => "Gauges",
            NavPage::Defects => "Defects",
            NavPage::Checklists => "Checklists",
            NavPage::Users => "People",
            NavPage::Audit => "Audit",
            NavPage::Webhooks => "Webhooks",
            NavPage::Reports => "Reports",
            NavPage::Search => "Search",
            NavPage::Stats => "Statistics",
            NavPage::Settings => "Settings",
        }
    }

    fn title(self) -> &'static str {
        match self {
            NavPage::Dashboard => "Shop",
            NavPage::Nonconformances => "Nonconformances",
            NavPage::CorrectiveActions => "Corrective actions",
            NavPage::Users => "People",
            NavPage::Audit => "Audit log",
            other => other.label(),
        }
    }

    fn lede(self) -> &'static str {
        match self {
            NavPage::Dashboard => "Squeeze chutes, corral gates, panels, wood stoves, and fireplaces.",
            NavPage::Lots => "A lot is one build. It waits, gets checked, then ships.",
            NavPage::Inspections => "A check against a build. Open one, or start the final on a lot.",
            NavPage::Nonconformances => "A weld, a latch, a dimension, or a hearth part that did not pass.",
            NavPage::CorrectiveActions => "The work that stops the same failure on the next build.",
            NavPage::Measurements => "Recorded widths, heights, lengths, and collar sizes.",
            NavPage::Products => "Squeeze chute, corral gate, corral panel, wood stove, and wood fireplace.",
            NavPage::Plans => "The final check for a product. Dimensions are compared to the print.",
            NavPage::Sites => "The shop.",
            NavPage::Stations => "Fit and weld, paint and assembly, and final inspection.",
            NavPage::Suppliers => "Where steel, glass, and hardware come from.",
            NavPage::Gauges => "Tape, caliper, square, and weld gauge.",
            NavPage::Defects => "Welds, edges, latches, finish, glass, gaskets, and plates.",
            NavPage::Checklists => "Cattle equipment final, and wood stove and fireplace final.",
            NavPage::Users => "Who can sign in, and what they are allowed to do.",
            NavPage::Audit => "Every write the server stored.",
            NavPage::Webhooks => "Where the server sends events.",
            NavPage::Reports => "Download the shop as a file.",
            NavPage::Search => "Find a chute, a gate, a stove, a check, or an alert.",
            NavPage::Stats => "How the shop has been doing.",
            NavPage::Settings => "Server, appearance, and the account on this Mac.",
        }
    }

    fn floor() -> &'static [NavPage] {
        &[
            NavPage::Dashboard,
            NavPage::Lots,
            NavPage::Inspections,
            NavPage::Nonconformances,
            NavPage::CorrectiveActions,
            NavPage::Measurements,
        ]
    }

    fn master() -> &'static [NavPage] {
        &[
            NavPage::Products,
            NavPage::Plans,
            NavPage::Checklists,
            NavPage::Sites,
            NavPage::Stations,
            NavPage::Suppliers,
            NavPage::Gauges,
            NavPage::Defects,
        ]
    }

    fn tools() -> &'static [NavPage] {
        &[
            NavPage::Search,
            NavPage::Stats,
            NavPage::Reports,
            NavPage::Users,
            NavPage::Audit,
            NavPage::Webhooks,
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

#[derive(Debug, Clone, Copy, PartialEq, Eq)]
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

impl Default for NcrForm {
    fn default() -> Self {
        Self {
            product_id: String::new(),
            title: String::new(),
            description: String::new(),
            severity: "minor".into(),
            quantity: "1".into(),
            error: None,
        }
    }
}

struct CapaForm {
    ncr: Option<(Uuid, String)>,
    title: String,
    root_cause: String,
    action_plan: String,
    due_date: String,
    error: Option<String>,
}

impl Default for CapaForm {
    fn default() -> Self {
        Self {
            ncr: None,
            title: String::new(),
            root_cause: String::new(),
            action_plan: String::new(),
            due_date: chrono::Local::now().date_naive().to_string(),
            error: None,
        }
    }
}

#[derive(Debug, Clone, Copy, PartialEq, Eq)]
enum ShotStep {
    WaitLogin,
    CaptureLogin,
    WaitDashboard,
    CaptureDashboard,
    CaptureInspections,
    CaptureWorkbench,
    CaptureNcrs,
    CaptureCapas,
    CaptureSettings,
    CaptureLots,
    CapturePlans,
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
    /// Unfiltered open alerts for the floor page; `ncrs` follows the alert list filter.
    floor_ncrs: Vec<Nonconformance>,
    capas: Vec<CorrectiveAction>,
    products: Vec<Product>,
    lots: Vec<Lot>,
    sites: Vec<Site>,
    defect_codes: Vec<DefectCode>,
    gauges: Vec<Gauge>,
    suppliers: Vec<serde_json::Value>,

    search: String,
    status_filter: String,
    sort: SortState,
    detail: DetailView,
    form: FormMode,
    insp_form: InspectionForm,
    ncr_form: NcrForm,
    capa_form: CapaForm,

    workbench: Option<Workbench>,
    ncr_detail: Option<NcrDetail>,
    capa_detail: Option<CorrectiveAction>,
    action_error: Option<String>,
    draft: ActionDraft,

    session_at: Option<Instant>,
    refresh_promise: Option<Promise<Result<TokenResponse, ApiError>>>,
    retry_after_refresh: bool,

    login_promise: Option<Promise<Result<TokenResponse, ApiError>>>,
    dash_promise: Option<Promise<Result<Dashboard, ApiError>>>,
    list_promise: Option<Promise<Result<ListPayload, ApiError>>>,
    detail_promise: Option<Promise<Result<DetailPayload, ApiError>>>,
    mutate_promise: Option<Promise<Result<MutatePayload, ApiError>>>,
    bootstrap_promise: Option<Promise<Result<BootstrapPayload, ApiError>>>,

    loading: bool,
    list_error: Option<String>,
    catalog_rows: Vec<serde_json::Value>,
    catalog_promise: Option<Promise<Result<Vec<serde_json::Value>, ApiError>>>,
    catalog_form: Vec<(String, String)>,
    catalog_form_page: Option<NavPage>,
    catalog_selected: Option<Uuid>,
    catalog_plan: Option<crate::api::PlanDetail>,
    plan_promise: Option<Promise<Result<crate::api::PlanDetail, ApiError>>>,
    char_form: catalog::CharForm,
    record_edit: Option<editor::RecordEdit>,
    item_form: catalog::ItemForm,
    /// Extra detail for the selected row, such as a checklist with its items.
    catalog_extra: Option<serde_json::Value>,
    extra_promise: Option<Promise<Result<serde_json::Value, ApiError>>>,
    stats_text: Option<String>,

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
    Inspection(Box<InspectionBundle>),
    Ncr(NcrDetail),
    Capa(CorrectiveAction),
}

enum MutatePayload {
    Ok(String),
    Stats(String),
    InspectionCreated(Box<InspectionDetail>),
    NcrSaved(Box<Nonconformance>),
    CapaSaved(Box<CorrectiveAction>),
    /// A status change on the open record: show the message and load it again.
    Reload(String),
    /// The open record was deleted: close it and reload the list.
    Deleted(String),
    Completed(Box<CompletedInspection>),
}

struct BootstrapPayload {
    products: Vec<Product>,
    lots: Vec<Lot>,
    ncrs: Vec<Nonconformance>,
    sites: Vec<Site>,
    defect_codes: Vec<DefectCode>,
    gauges: Vec<Gauge>,
    suppliers: Vec<serde_json::Value>,
}

impl QcApp {
    pub fn new(cc: &eframe::CreationContext<'_>, opts: LaunchOptions) -> Self {
        chrome::install_fonts(&cc.egui_ctx);
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
            floor_ncrs: Vec::new(),
            capas: Vec::new(),
            products: Vec::new(),
            lots: Vec::new(),
            sites: Vec::new(),
            defect_codes: Vec::new(),
            gauges: Vec::new(),
            suppliers: Vec::new(),
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
            ncr_form: NcrForm::default(),
            capa_form: CapaForm::default(),
            workbench: None,
            ncr_detail: None,
            capa_detail: None,
            action_error: None,
            draft: ActionDraft::default(),
            session_at: None,
            refresh_promise: None,
            retry_after_refresh: false,
            login_promise: None,
            dash_promise: None,
            list_promise: None,
            detail_promise: None,
            mutate_promise: None,
            bootstrap_promise: None,
            loading: false,
            list_error: None,
            catalog_rows: Vec::new(),
            catalog_promise: None,
            catalog_form: Vec::new(),
            catalog_form_page: None,
            catalog_selected: None,
            catalog_plan: None,
            plan_promise: None,
            char_form: catalog::CharForm::default(),
            record_edit: None,
            item_form: catalog::ItemForm::default(),
            catalog_extra: None,
            extra_promise: None,
            stats_text: None,
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
        self.spawn_bootstrap();
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
        if self.bootstrap_promise.is_some() {
            return;
        }
        self.bootstrap_promise = Some(Promise::spawn_thread("bootstrap", move || {
            let (products, _) = client.list_products(None)?;
            let (lots, _) = client.list_lots(None)?;
            let (ncrs, _) = client.list_ncrs(None, 200, 0)?;
            let (sites, _) = client.list_sites()?;
            let defect_codes = client.list_defect_codes()?;
            let gauges = client.list_gauges()?;
            let suppliers = client.list_json("/api/v1/suppliers").unwrap_or_default();
            Ok(BootstrapPayload {
                products,
                lots,
                ncrs,
                sites,
                defect_codes,
                gauges,
                suppliers,
            })
        }));
    }

    fn spawn_detail(&mut self, detail: DetailView) {
        let client = self.authed_client();
        self.detail_promise = Some(Promise::spawn_thread("detail", move || match detail {
            DetailView::Inspection(id) => {
                let detail = client.get_inspection(id)?;
                let plan = client.get_plan(detail.inspection.plan_id).ok();
                let lot = client.get_lot(detail.inspection.lot_id).ok();
                Ok(DetailPayload::Inspection(Box::new(InspectionBundle { detail, plan, lot })))
            }
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
                        self.session_at = Some(Instant::now());
                        self.login.password.clear();
                        self.login.error = None;
                        self.page = NavPage::Dashboard;
                        self.spawn_dashboard();
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
                    Err(e) => self.list_failed(e),
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
                    Err(e) => self.list_failed(e),
                }
                ctx.request_repaint();
            }
        }

        if self
            .catalog_promise
            .as_ref()
            .is_some_and(|p| p.ready().is_some())
        {
            if let Some(p) = self.catalog_promise.take() {
                self.loading = false;
                match p.block_and_take() {
                    Ok(rows) => {
                        self.catalog_rows = rows;
                        self.list_error = None;
                    }
                    Err(e) => self.list_failed(e),
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
                match p.block_and_take() {
                    Ok(DetailPayload::Inspection(b)) => {
                        if self.detail == DetailView::Inspection(b.detail.inspection.id) {
                            match &mut self.workbench {
                                Some(wb) => wb.absorb(*b),
                                None => self.workbench = Some(Workbench::new(*b)),
                            }
                        }
                    }
                    Ok(DetailPayload::Ncr(d)) => {
                        if self.detail == DetailView::Ncr(d.ncr.id) {
                            self.ncr_detail = Some(d);
                        }
                    }
                    Ok(DetailPayload::Capa(d)) => {
                        if self.detail == DetailView::Capa(d.id) {
                            self.capa_detail = Some(d);
                        }
                    }
                    Err(e) => {
                        if e.is_unauthorized() {
                            self.renew_session(true);
                        }
                        self.action_error = Some(e.message());
                    }
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
                        self.catalog_form_page = None;
                        self.refresh_current();
                        if let (NavPage::Plans, Some(id)) = (self.page, self.catalog_selected) {
                            self.catalog_plan = None;
                            let client = self.authed_client();
                            self.plan_promise =
                                Some(Promise::spawn_thread("plan", move || client.get_plan(id)));
                        }
                        if let (NavPage::Checklists, Some(id)) = (self.page, self.catalog_selected) {
                            self.spawn_extra(id);
                        }
                        self.spawn_bootstrap();
                    }
                    Ok(MutatePayload::Stats(text)) => {
                        self.stats_text = Some(text);
                        self.list_error = None;
                    }
                    Ok(MutatePayload::InspectionCreated(d)) => {
                        self.status_msg =
                            Some(format!("{} is open. Record the checks below.", d.inspection.inspection_number));
                        self.insp_form.notes.clear();
                        self.show_detail(DetailView::Inspection(d.inspection.id));
                        self.spawn_bootstrap();
                    }
                    Ok(MutatePayload::NcrSaved(n)) => {
                        self.status_msg = Some(format!("{} saved.", n.ncr_number));
                        self.ncr_form = NcrForm::default();
                        self.show_detail(DetailView::Ncr(n.id));
                        self.spawn_bootstrap();
                    }
                    Ok(MutatePayload::CapaSaved(c)) => {
                        self.status_msg = Some(format!("{} saved.", c.ca_number));
                        self.capa_form = CapaForm::default();
                        self.show_detail(DetailView::Capa(c.id));
                    }
                    Ok(MutatePayload::Reload(msg)) => {
                        self.status_msg = Some(msg);
                        self.action_error = None;
                        self.draft = ActionDraft::default();
                        self.spawn_detail(self.detail);
                        self.spawn_bootstrap();
                    }
                    Ok(MutatePayload::Deleted(msg)) => {
                        self.status_msg = Some(msg);
                        self.detail = DetailView::None;
                        self.workbench = None;
                        self.ncr_detail = None;
                        self.capa_detail = None;
                        self.action_error = None;
                        self.draft = ActionDraft::default();
                        self.refresh_current();
                        self.spawn_bootstrap();
                    }
                    Ok(MutatePayload::Completed(done)) => {
                        let insp = &done.inspection;
                        let mut msg = match insp.result.as_str() {
                            "pass" => format!("{} passed. The lot can ship.", insp.inspection_number),
                            "conditional" => format!("{} passed with minor defects.", insp.inspection_number),
                            _ => format!("{} failed.", insp.inspection_number),
                        };
                        if let Some(n) = &done.nonconformance {
                            msg.push_str(&format!(" Alert {} is open.", n.ncr_number));
                        }
                        self.status_msg = Some(msg);
                        self.action_error = None;
                        self.spawn_detail(self.detail);
                        self.bootstrap_promise = None;
                        self.spawn_bootstrap();
                    }
                    Err(e) => {
                        if e.is_unauthorized() {
                            self.renew_session(false);
                        }
                        let msg = e.message();
                        match &mut self.form {
                            FormMode::CreateInspection => self.insp_form.error = Some(msg),
                            FormMode::CreateNcr | FormMode::EditNcr(_) => {
                                self.ncr_form.error = Some(msg)
                            }
                            FormMode::CreateCapa | FormMode::EditCapa(_) => {
                                self.capa_form.error = Some(msg)
                            }
                            FormMode::None if self.detail != DetailView::None => {
                                self.action_error = Some(msg)
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
                match p.block_and_take() {
                    Ok(b) => {
                        self.products = b.products;
                        self.lots = b.lots;
                        self.floor_ncrs = b.ncrs;
                        self.sites = b.sites;
                        self.defect_codes = b.defect_codes;
                        self.gauges = b.gauges;
                        self.suppliers = b.suppliers;
                    }
                    Err(e) if e.is_unauthorized() => self.renew_session(true),
                    Err(e) => self.list_error = Some(e.message()),
                }
                ctx.request_repaint();
            }
        }

        if self
            .refresh_promise
            .as_ref()
            .is_some_and(|p| p.ready().is_some())
        {
            if let Some(p) = self.refresh_promise.take() {
                match p.block_and_take() {
                    Ok(fresh) => {
                        self.client.set_access_token(Some(fresh.access_token.clone()));
                        self.session = Some(fresh);
                        self.session_at = Some(Instant::now());
                        if std::mem::take(&mut self.retry_after_refresh) {
                            self.list_error = None;
                            self.action_error = None;
                            self.refresh_current();
                            if self.detail != DetailView::None {
                                self.spawn_detail(self.detail);
                            }
                        }
                    }
                    Err(e) if matches!(e, ApiError::Transport(_)) => {
                        self.list_error = Some(e.message());
                    }
                    Err(_) => {
                        self.sign_out();
                        self.login.error = Some("You were signed out. Sign in again.".into());
                    }
                }
                ctx.request_repaint();
            }
        }

        let busy = self.dash_promise.is_some()
            || self.list_promise.is_some()
            || self.catalog_promise.is_some()
            || self.detail_promise.is_some()
            || self.mutate_promise.is_some()
            || self.login_promise.is_some();
        self.loading = busy;
        if busy || self.bootstrap_promise.is_some() || self.refresh_promise.is_some() {
            ctx.request_repaint_after(Duration::from_millis(100));
        }
    }

    fn poll_all(&mut self, ctx: &egui::Context) {
        self.poll_promises(ctx);
        self.poll_plan();
        self.poll_extra();
    }

    fn list_failed(&mut self, e: ApiError) {
        if e.is_unauthorized() {
            self.renew_session(true);
            return;
        }
        self.list_error = Some(e.message());
    }

    /// Trades the refresh token for a new access token. With `retry`, reloads what failed.
    fn renew_session(&mut self, retry: bool) {
        self.retry_after_refresh |= retry;
        if self.refresh_promise.is_some() {
            return;
        }
        let Some(s) = &self.session else {
            return;
        };
        let token = s.refresh_token.clone();
        let mut client = self.client.clone();
        client.set_base_url(self.settings.server_url.clone());
        client.set_access_token(None);
        self.refresh_promise = Some(Promise::spawn_thread("refresh", move || client.refresh(&token)));
    }

    fn keep_session_fresh(&mut self) {
        let (Some(s), Some(at)) = (&self.session, self.session_at) else {
            return;
        };
        let life = Duration::from_secs(s.expires_in.max(60) as u64);
        if at.elapsed() >= life.mul_f32(0.75) {
            self.renew_session(false);
        }
    }

    fn sign_out(&mut self) {
        if let Some(s) = self.session.take() {
            let client = self.authed_client_for(&s);
            let token = s.refresh_token;
            std::thread::spawn(move || {
                let _ = client.logout(&token);
            });
        }
        self.client.set_access_token(None);
        self.session_at = None;
        self.refresh_promise = None;
        self.retry_after_refresh = false;
        self.dashboard = None;
        self.inspections.clear();
        self.ncrs.clear();
        self.floor_ncrs.clear();
        self.capas.clear();
        self.lots.clear();
        self.products.clear();
        self.catalog_rows.clear();
        self.workbench = None;
        self.ncr_detail = None;
        self.capa_detail = None;
        self.detail = DetailView::None;
        self.form = FormMode::None;
        self.status_msg = None;
        self.list_error = None;
        self.action_error = None;
        self.page = NavPage::Dashboard;
    }

    fn authed_client_for(&self, s: &TokenResponse) -> ApiClient {
        let mut c = self.client.clone();
        c.set_base_url(self.settings.server_url.clone());
        c.set_access_token(Some(s.access_token.clone()));
        c
    }

    /// Opens a record on its own page, dropping whatever detail was showing.
    fn show_detail(&mut self, view: DetailView) {
        let page = match view {
            DetailView::Inspection(_) => NavPage::Inspections,
            DetailView::Ncr(_) => NavPage::Nonconformances,
            DetailView::Capa(_) => NavPage::CorrectiveActions,
            DetailView::None => self.page,
        };
        self.goto(page);
        self.form = FormMode::None;
        self.workbench = None;
        self.ncr_detail = None;
        self.capa_detail = None;
        self.action_error = None;
        self.draft = ActionDraft::default();
        self.detail = view;
        self.spawn_detail(view);
    }

    fn refresh_current(&mut self) {
        match self.page {
            NavPage::Dashboard => self.spawn_dashboard(),
            NavPage::Inspections | NavPage::Nonconformances | NavPage::CorrectiveActions => {
                self.spawn_list()
            }
            NavPage::Settings | NavPage::Reports => {}
            NavPage::Stats => self.spawn_stats(),
            page if page.catalog().is_some() => self.spawn_catalog(),
            _ => {}
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
        if esc && !ctx.wants_keyboard_input() {
            if !matches!(self.form, FormMode::None) {
                self.form = FormMode::None;
            } else if self.detail != DetailView::None {
                self.detail = DetailView::None;
                self.workbench = None;
                self.ncr_detail = None;
                self.capa_detail = None;
                self.action_error = None;
                self.refresh_current();
            }
        }
        if mod_key && r {
            self.refresh_current();
        }
        if mod_key && comma {
            self.page = NavPage::Settings;
        }
        if mod_key && n {
            match self.page {
                NavPage::Inspections => self.open_new_form(FormMode::CreateInspection),
                NavPage::Nonconformances => self.open_new_form(FormMode::CreateNcr),
                NavPage::CorrectiveActions => self.open_new_form(FormMode::CreateCapa),
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

    fn nav_group(&mut self, ui: &mut egui::Ui, title: &str, pages: &[NavPage]) {
        chrome::nav_heading(ui, title);
        for page in pages {
            if chrome::nav_row(ui, page.label(), self.page == *page).clicked() {
                self.goto(*page);
            }
        }
    }

    fn goto(&mut self, page: NavPage) {
        if self.page != page {
            self.page = page;
            self.detail = DetailView::None;
            self.form = FormMode::None;
            self.search.clear();
            self.status_filter.clear();
            self.catalog_rows.clear();
            self.catalog_selected = None;
            self.catalog_plan = None;
            self.catalog_extra = None;
            self.record_edit = None;
            self.list_error = None;
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

    fn open_new_form(&mut self, mode: FormMode) {
        match mode {
            FormMode::CreateNcr => self.ncr_form = NcrForm::default(),
            FormMode::CreateCapa => self.capa_form = CapaForm::default(),
            FormMode::CreateInspection => self.insp_form.error = None,
            _ => {}
        }
        self.form = mode;
    }

    fn submit_inspection(&mut self) {
        if self.mutate_promise.is_some() {
            return;
        }
        let lot_id = match Uuid::parse_str(self.insp_form.lot_id.trim()) {
            Ok(id) => id,
            Err(_) => {
                self.insp_form.error = Some("Pick the build to check.".into());
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
            Ok(MutatePayload::InspectionCreated(Box::new(d)))
        }));
    }

    fn submit_ncr(&mut self) {
        if self.mutate_promise.is_some() {
            return;
        }
        let product_id = match Uuid::parse_str(self.ncr_form.product_id.trim()) {
            Ok(id) => id,
            Err(_) => {
                self.ncr_form.error = Some("Pick the product.".into());
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
                    Ok(MutatePayload::NcrSaved(Box::new(d)))
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
                    Ok(MutatePayload::NcrSaved(Box::new(d)))
                }));
            }
            _ => {}
        }
    }

    fn submit_capa(&mut self) {
        if self.mutate_promise.is_some() {
            return;
        }
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
                    nonconformance_id: self.capa_form.ncr.as_ref().map(|(id, _)| *id),
                    title: self.capa_form.title.clone(),
                    root_cause: self.capa_form.root_cause.clone(),
                    action_plan: self.capa_form.action_plan.clone(),
                    owner_id: None,
                    due_date: self.capa_form.due_date.clone(),
                };
                self.mutate_promise = Some(Promise::spawn_thread("create-capa", move || {
                    let d = client.create_capa(&body)?;
                    Ok(MutatePayload::CapaSaved(Box::new(d)))
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
                    Ok(MutatePayload::CapaSaved(Box::new(d)))
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
                let ready = self.logged_in() && self.dashboard.is_some() && self.shot_frames > 12;
                if ready || self.shot_frames > 180 {
                    self.shot_step = Some(ShotStep::CaptureDashboard);
                    self.shot_frames = 0;
                }
            }
            ShotStep::CaptureDashboard => {
                self.page = NavPage::Dashboard;
                if self.shot_frames == 2 {
                    self.request_screenshot(ctx, "02_dashboard");
                } else if self.shot_frames > 6 && self.pending_shot_name.is_none() {
                    self.shot_step = Some(ShotStep::CaptureInspections);
                    self.shot_frames = 0;
                    self.goto(NavPage::Inspections);
                }
            }
            ShotStep::CaptureInspections => {
                if self.page != NavPage::Inspections {
                    self.goto(NavPage::Inspections);
                }
                if self.shot_frames == 20 {
                    self.request_screenshot(ctx, "03_inspections");
                } else if self.shot_frames > 24 && self.pending_shot_name.is_none() {
                    self.shot_step = Some(ShotStep::CaptureWorkbench);
                    self.shot_frames = 0;
                    let open = self
                        .inspections
                        .iter()
                        .find(|i| i.status == "in_progress")
                        .or(self.inspections.first())
                        .map(|i| i.id);
                    if let Some(id) = open {
                        self.show_detail(DetailView::Inspection(id));
                    }
                }
            }
            ShotStep::CaptureWorkbench => {
                let ready = self.workbench.is_some() && self.shot_frames > 10;
                if (ready || self.shot_frames > 120) && self.pending_shot_name.is_none() && self.shot_frames < 1000 {
                    self.request_screenshot(ctx, "03b_workbench");
                    self.shot_frames = 1000;
                } else if self.shot_frames > 1004 && self.pending_shot_name.is_none() {
                    self.shot_step = Some(ShotStep::CaptureNcrs);
                    self.shot_frames = 0;
                    self.goto(NavPage::Nonconformances);
                }
            }
            ShotStep::CaptureNcrs => {
                if self.shot_frames == 20 {
                    self.request_screenshot(ctx, "04_nonconformances");
                } else if self.shot_frames == 24 {
                    if let Some(id) = self.ncrs.first().map(|n| n.id) {
                        self.show_detail(DetailView::Ncr(id));
                    }
                } else if self.shot_frames == 50 {
                    self.request_screenshot(ctx, "04b_alert");
                } else if self.shot_frames > 54 && self.pending_shot_name.is_none() {
                    self.shot_step = Some(ShotStep::CaptureCapas);
                    self.shot_frames = 0;
                    self.goto(NavPage::CorrectiveActions);
                }
            }
            ShotStep::CaptureCapas => {
                if self.shot_frames == 20 {
                    self.request_screenshot(ctx, "05_corrective_actions");
                } else if self.shot_frames == 24 {
                    if let Some(id) = self.capas.first().map(|c| c.id) {
                        self.show_detail(DetailView::Capa(id));
                    }
                } else if self.shot_frames == 50 {
                    self.request_screenshot(ctx, "05b_action");
                } else if self.shot_frames > 54 && self.pending_shot_name.is_none() {
                    self.shot_step = Some(ShotStep::CaptureSettings);
                    self.shot_frames = 0;
                    self.goto(NavPage::Settings);
                }
            }
            ShotStep::CaptureSettings => {
                if self.shot_frames == 12 {
                    self.request_screenshot(ctx, "06_settings");
                } else if self.shot_frames > 16 && self.pending_shot_name.is_none() {
                    self.shot_step = Some(ShotStep::CaptureLots);
                    self.shot_frames = 0;
                    self.goto(NavPage::Lots);
                }
            }
            ShotStep::CaptureLots => {
                if self.shot_frames == 20 {
                    if let Some(id) = self.catalog_rows.first().and_then(|r| r.get("id")).and_then(|v| v.as_str()) {
                        self.catalog_selected = Uuid::parse_str(id).ok();
                    }
                } else if self.shot_frames == 26 {
                    self.request_screenshot(ctx, "07_lots");
                } else if self.shot_frames > 30 && self.pending_shot_name.is_none() {
                    self.shot_step = Some(ShotStep::CapturePlans);
                    self.shot_frames = 0;
                    self.goto(NavPage::Plans);
                }
            }
            ShotStep::CapturePlans => {
                if self.shot_frames == 20 {
                    if let Some(id) = self.catalog_rows.first().and_then(|r| r.get("id")).and_then(|v| v.as_str()) {
                        if let Ok(id) = Uuid::parse_str(id) {
                            self.catalog_selected = Some(id);
                            let client = self.authed_client();
                            self.plan_promise = Some(Promise::spawn_thread("plan", move || client.get_plan(id)));
                        }
                    }
                } else if self.shot_frames == 40 {
                    self.request_screenshot(ctx, "08_plans");
                } else if self.shot_frames > 44 && self.pending_shot_name.is_none() {
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
        self.poll_all(ctx);
        self.keep_session_fresh();
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

        let sidebar = chrome::colors(ctx.style().visuals.dark_mode).sidebar;
        egui::SidePanel::left("sidebar")
            .resizable(false)
            .exact_width(248.0)
            .show_separator_line(false)
            .frame(
                egui::Frame::none()
                    .fill(sidebar)
                    .inner_margin(egui::Margin::symmetric(12.0, 16.0)),
            )
            .show(ctx, |ui| {
                self.ui_sidebar(ui);
            });

        let canvas = chrome::colors(ctx.style().visuals.dark_mode).canvas;
        egui::CentralPanel::default()
            .frame(
                egui::Frame::none()
                    .fill(canvas)
                    .inner_margin(egui::Margin::same(28.0)),
            )
            .show(ctx, |ui| {
                self.page_header(ui);
                match self.page {
                    NavPage::Dashboard => self.ui_dashboard(ui),
                    NavPage::Inspections => self.ui_inspections(ui),
                    NavPage::Nonconformances => self.ui_ncrs(ui),
                    NavPage::CorrectiveActions => self.ui_capas(ui),
                    NavPage::Settings => self.ui_settings(ui),
                    _ => self.ui_catalog(ui),
                }
            });
    }
}

impl QcApp {
    fn ui_sidebar(&mut self, ui: &mut egui::Ui) {
        let p = chrome::colors(true);
        ui.horizontal(|ui| {
            let (mark, _) = ui.allocate_exact_size(egui::vec2(28.0, 28.0), egui::Sense::hover());
            ui.painter()
                .rect_filled(mark, egui::Rounding::same(8.0), p.teal);
            ui.painter().text(
                mark.center(),
                egui::Align2::CENTER_CENTER,
                "Q",
                chrome::semibold_id(14.0),
                egui::Color32::WHITE,
            );
            ui.vertical(|ui| {
                ui.label(chrome::semi("Quality Control", 14.5, p.sidebar_ink));
                ui.label(
                    egui::RichText::new("Cattle gear and hearth")
                        .size(11.5)
                        .color(p.sidebar_muted),
                );
            });
        });
        let footer = 78.0;
        let scroll_h = (ui.available_height() - footer).max(120.0);
        ui.allocate_ui(egui::vec2(ui.available_width(), scroll_h), |ui| {
            egui::ScrollArea::vertical()
                .id_salt("nav-scroll")
                .show(ui, |ui| {
                    self.nav_group(ui, "Shop", NavPage::floor());
                    self.nav_group(ui, "Records", NavPage::master());
                    self.nav_group(ui, "Tools", NavPage::tools());
                    ui.add_space(12.0);
                });
        });
        ui.add_space(8.0);
        if let Some(s) = &self.session {
            let name = if s.user.full_name.trim().is_empty() {
                s.user.username.clone()
            } else {
                s.user.full_name.clone()
            };
            ui.label(chrome::semi(name, 13.0, p.sidebar_ink));
            ui.label(
                egui::RichText::new(s.user.role.replace('_', " "))
                    .size(12.0)
                    .color(p.sidebar_muted),
            );
        }
        let (rect, response) = ui.allocate_exact_size(
            egui::vec2(ui.available_width(), 28.0),
            egui::Sense::click(),
        );
        let color = if response.hovered() {
            p.sidebar_ink
        } else {
            p.sidebar_muted
        };
        ui.painter().text(
            rect.left_center(),
            egui::Align2::LEFT_CENTER,
            "Sign out",
            egui::FontId::proportional(13.0),
            color,
        );
        if response.clicked() {
            self.sign_out();
        }
    }

    fn page_header(&mut self, ui: &mut egui::Ui) {
        let p = chrome::palette(ui);
        ui.horizontal(|ui| {
            ui.vertical(|ui| {
                ui.label(chrome::semi(self.page.title(), 26.0, p.ink));
                ui.add_space(2.0);
                ui.label(egui::RichText::new(self.page.lede()).size(13.5).color(p.muted));
            });
            ui.with_layout(egui::Layout::right_to_left(egui::Align::Center), |ui| {
                ui.spacing_mut().item_spacing.x = 8.0;
                if self.loading {
                    ui.spinner();
                }
                match self.page {
                    NavPage::Dashboard => {
                        if chrome::button(ui, "Refresh", Btn::Quiet).clicked() {
                            self.refresh_current();
                        }
                    }
                    NavPage::Inspections | NavPage::Nonconformances | NavPage::CorrectiveActions => {
                        let (primary, mode) = match self.page {
                            NavPage::Inspections => ("New inspection", FormMode::CreateInspection),
                            NavPage::Nonconformances => ("New alert", FormMode::CreateNcr),
                            _ => ("New action", FormMode::CreateCapa),
                        };
                        if chrome::button(ui, primary, Btn::Primary).clicked() {
                            self.open_new_form(mode);
                        }
                        if chrome::button(ui, "Export", Btn::Quiet).clicked() {
                            self.export_current();
                        }
                        if chrome::button(ui, "Refresh", Btn::Quiet).clicked() {
                            self.refresh_current();
                        }
                    }
                    page if page.catalog().is_some()
                        && chrome::button(ui, "Refresh", Btn::Quiet).clicked() =>
                    {
                        self.refresh_current();
                    }
                    _ => {}
                }
            });
        });
        ui.add_space(14.0);
        if let Some(msg) = &self.status_msg.clone() {
            chrome::banner(ui, msg, false);
            ui.add_space(8.0);
        }
        if let Some(err) = &self.list_error.clone() {
            error_banner(ui, err);
            ui.add_space(8.0);
        }
    }

    fn ui_login(&mut self, ctx: &egui::Context) {
        let p = chrome::colors(ctx.style().visuals.dark_mode);
        let side = chrome::colors(true);
        egui::CentralPanel::default()
            .frame(egui::Frame::none().fill(p.canvas))
            .show(ctx, |ui| {
                let full = ui.available_rect_before_wrap();
                let split = (full.width() * 0.42).clamp(420.0, 560.0);
                let left = egui::Rect::from_min_size(full.min, egui::vec2(split, full.height()));
                ui.painter().rect_filled(left, egui::Rounding::ZERO, side.sidebar);
                let mut brand = ui.new_child(
                    egui::UiBuilder::new()
                        .id_salt("login-brand")
                        .max_rect(left.shrink(48.0)),
                );
                brand.add_space(36.0);
                brand.label(chrome::semi("QUALITY CONTROL", 12.0, side.teal).extra_letter_spacing(1.2));
                brand.add_space(16.0);
                brand.label(chrome::semi("Cattle gear\nand hearth.", 40.0, side.sidebar_ink));
                brand.add_space(16.0);
                brand.label(
                    egui::RichText::new(
                        "Squeeze chutes, corral gates, panels, wood stoves, and fireplaces. The checks for those builds live here.",
                    )
                    .size(15.0)
                    .color(side.sidebar_muted),
                );

                let right = egui::Rect::from_min_max(
                    egui::pos2(left.right(), full.top()),
                    full.max,
                );
                let mut pane = ui.new_child(
                    egui::UiBuilder::new()
                        .id_salt("login-pane")
                        .max_rect(right),
                );
                let card_w = 400.0;
                let card_h = 460.0;
                pane.add_space(((right.height() - card_h) / 2.0).max(24.0));
                pane.horizontal(|ui| {
                    ui.add_space(((right.width() - card_w) / 2.0).max(24.0));
                    ui.allocate_ui(egui::vec2(card_w, card_h), |ui| {
                    chrome::surface(ui, "login-card", card_h, |ui| {
                        ui.label(chrome::semi("Sign in", 24.0, p.ink));
                        ui.add_space(4.0);
                        ui.label(
                            egui::RichText::new("Use the app account, not the database login.")
                                .color(p.muted)
                                .size(13.0),
                        );
                        ui.add_space(18.0);
                        chrome::field_label(ui, "Server");
                        chrome::text_line(ui, &mut self.settings.server_url);
                        ui.add_space(10.0);
                        chrome::field_label(ui, "Username");
                        chrome::text_line(ui, &mut self.login.username);
                        ui.add_space(10.0);
                        chrome::field_label(ui, "Password");
                        ui.add(
                            egui::TextEdit::singleline(&mut self.login.password)
                                .password(true)
                                .desired_width(f32::INFINITY)
                                .margin(egui::Margin::symmetric(10.0, 8.0)),
                        );
                        ui.add_space(18.0);
                        let enter = ui.input(|i| i.key_pressed(egui::Key::Enter));
                        let clicked = ui
                            .add_enabled_ui(!self.loading, |ui| {
                                chrome::button_fill(ui, "Sign in", Btn::Primary).clicked()
                            })
                            .inner;
                        if (clicked || enter)
                            && !self.loading
                            && !self.login.username.is_empty()
                            && !self.login.password.is_empty()
                        {
                            self.client.set_base_url(self.settings.server_url.clone());
                            let _ = self.settings.save();
                            self.spawn_login();
                        }
                        if self.loading {
                            ui.add_space(10.0);
                            loading_spinner(ui);
                        }
                        if let Some(err) = &self.login.error {
                            ui.add_space(10.0);
                            error_banner(ui, err);
                        }
                    });
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
            empty_state(ui, "The bench is empty.", "Refresh once the API is signed in.");
            return;
        };
        let lots = self.lots.clone();
        let ncrs = self.floor_ncrs.clone();
        if let Some(action) = bench::draw_today(ui, &d, &lots, &ncrs) {
            match action {
                bench::BenchAction::Inspect(id) => {
                    self.insp_form.lot_id = id.to_string();
                    self.goto(NavPage::Inspections);
                    self.form = FormMode::CreateInspection;
                }
                bench::BenchAction::Alerts => self.goto(NavPage::Nonconformances),
                bench::BenchAction::NewLot => self.goto(NavPage::Lots),
                bench::BenchAction::NewCheck => {
                    self.goto(NavPage::Inspections);
                    self.form = FormMode::CreateInspection;
                }
                bench::BenchAction::OpenNcr(id) => self.show_detail(DetailView::Ncr(id)),
            }
        }
    }

    fn ui_inspections(&mut self, ui: &mut egui::Ui) {
        if matches!(self.form, FormMode::CreateInspection) {
            self.ui_inspection_form(ui);
        }
        if let DetailView::Inspection(_) = self.detail {
            self.ui_inspection_detail(ui);
            return;
        }
        self.ui_toolbar(ui, &["", "in_progress", "completed", "cancelled"]);
        let rows: Vec<Inspection> = self.filtered_inspections().into_iter().cloned().collect();
        if !self.loading && rows.is_empty() {
            empty_state(
                ui,
                "No inspections match.",
                "Change the filter, or start a check with New inspection.",
            );
            return;
        }
        if let Some(title) = record_table(
            ui,
            &["Number", "Build", "Status", "Result", "Started", ""],
            rows.len(),
            |ui, idx, col| match col {
                0 => {
                    ui.label(&rows[idx].inspection_number);
                }
                1 => {
                    let build = self
                        .lots
                        .iter()
                        .find(|l| l.id == rows[idx].lot_id)
                        .map(|l| {
                            let product = self
                                .products
                                .iter()
                                .find(|p| p.id == l.product_id)
                                .map(|p| p.name.as_str())
                                .unwrap_or("");
                            format!("{}  {}", l.lot_number, product)
                        })
                        .unwrap_or_default();
                    ui.add(egui::Label::new(build).truncate());
                }
                2 => status_badge(ui, &rows[idx].status),
                3 => status_badge(ui, &rows[idx].result),
                4 => {
                    ui.label(rows[idx].started_at.with_timezone(&chrono::Local).format("%b %-d, %H:%M").to_string());
                }
                _ => {
                    let label = if rows[idx].status == "in_progress" { "Continue" } else { "Open" };
                    if quiet_button(ui, label).clicked() {
                        self.show_detail(DetailView::Inspection(rows[idx].id));
                    }
                }
            },
        ) {
            if let Some(key) = column_sort(&title) {
                self.toggle_sort(key);
            }
        }
    }

    fn ui_ncrs(&mut self, ui: &mut egui::Ui) {
        if matches!(self.form, FormMode::CreateNcr | FormMode::EditNcr(_)) {
            self.ui_ncr_form(ui);
        }
        if let DetailView::Ncr(_) = self.detail {
            self.ui_ncr_detail(ui);
            return;
        }
        self.ui_toolbar(
            ui,
            &["", "open", "under_review", "dispositioned", "closed", "void"],
        );
        let rows: Vec<Nonconformance> = self.filtered_ncrs().into_iter().cloned().collect();
        if !self.loading && rows.is_empty() {
            empty_state(
                ui,
                "No alerts match.",
                "Change the filter, or create one with New alert.",
            );
            return;
        }
        if let Some(title) = record_table(
            ui,
            &["Number", "Title", "Severity", "Status", ""],
            rows.len(),
            |ui, idx, col| match col {
                0 => {
                    ui.label(&rows[idx].ncr_number);
                }
                1 => {
                    ui.label(&rows[idx].title);
                }
                2 => status_badge(ui, &rows[idx].severity),
                3 => status_badge(ui, &rows[idx].status),
                _ => {
                    ui.horizontal(|ui| {
                        if quiet_button(ui, "Open").clicked() {
                            self.show_detail(DetailView::Ncr(rows[idx].id));
                        }
                        let editable = matches!(rows[idx].status.as_str(), "open" | "under_review" | "dispositioned");
                        if editable && quiet_button(ui, "Edit").clicked() {
                            self.ncr_form.product_id = rows[idx].product_id.to_string();
                            self.ncr_form.title = rows[idx].title.clone();
                            self.ncr_form.description = rows[idx].description.clone();
                            self.ncr_form.severity = rows[idx].severity.clone();
                            self.ncr_form.quantity = rows[idx].quantity_affected.to_string();
                            self.form = FormMode::EditNcr(rows[idx].id);
                        }
                    });
                }
            },
        ) {
            if let Some(key) = column_sort(&title) {
                self.toggle_sort(key);
            }
        }
    }

    fn ui_capas(&mut self, ui: &mut egui::Ui) {
        if matches!(self.form, FormMode::CreateCapa | FormMode::EditCapa(_)) {
            self.ui_capa_form(ui);
        }
        if let DetailView::Capa(_) = self.detail {
            self.ui_capa_detail(ui);
            return;
        }
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
        let rows: Vec<CorrectiveAction> = self.filtered_capas().into_iter().cloned().collect();
        if !self.loading && rows.is_empty() {
            empty_state(
                ui,
                "No actions match.",
                "Change the filter, or create one with New action.",
            );
            return;
        }
        if let Some(title) = record_table(
            ui,
            &["Number", "Title", "Status", "Due", ""],
            rows.len(),
            |ui, idx, col| match col {
                0 => {
                    ui.label(&rows[idx].ca_number);
                }
                1 => {
                    ui.label(&rows[idx].title);
                }
                2 => status_badge(ui, &rows[idx].status),
                3 => {
                    ui.label(&rows[idx].due_date);
                }
                _ => {
                    ui.horizontal(|ui| {
                        if quiet_button(ui, "Open").clicked() {
                            self.show_detail(DetailView::Capa(rows[idx].id));
                        }
                        let editable = matches!(rows[idx].status.as_str(), "open" | "in_progress" | "verification");
                        if editable && quiet_button(ui, "Edit").clicked() {
                            self.capa_form.title = rows[idx].title.clone();
                            self.capa_form.root_cause = rows[idx].root_cause.clone();
                            self.capa_form.action_plan = rows[idx].action_plan.clone();
                            self.capa_form.due_date = rows[idx].due_date.clone();
                            self.form = FormMode::EditCapa(rows[idx].id);
                        }
                    });
                }
            },
        ) {
            if let Some(key) = column_sort(&title) {
                self.toggle_sort(key);
            }
        }
    }

    fn ui_settings(&mut self, ui: &mut egui::Ui) {
        egui::ScrollArea::vertical().show(ui, |ui| {
            chrome::form_card(ui, "Connection", |ui| {
                chrome::field_label(ui, "Server URL");
                chrome::text_line(ui, &mut self.settings.server_url);
                ui.add_space(8.0);
                ui.label(
                    egui::RichText::new(
                        "The app talks to this address. The database port is not this address.",
                    )
                    .color(chrome::palette(ui).muted)
                    .size(12.5),
                );
            });
            ui.add_space(12.0);
            chrome::form_card(ui, "Appearance", |ui| {
                ui.horizontal(|ui| {
                    ui.radio_value(&mut self.settings.theme, ThemePreference::Light, "Light");
                    ui.radio_value(&mut self.settings.theme, ThemePreference::Dark, "Dark");
                    ui.radio_value(&mut self.settings.theme, ThemePreference::System, "System");
                });
            });
            ui.add_space(12.0);
            if chrome::button(ui, "Save settings", Btn::Primary).clicked() {
                self.client.set_base_url(self.settings.server_url.clone());
                match self.settings.save() {
                    Ok(()) => {
                        self.status_msg = Some(format!("Saved to {}", AppSettings::path_display()))
                    }
                    Err(e) => self.list_error = Some(e),
                }
            }
            ui.add_space(12.0);
            chrome::form_card(ui, "This desk", |ui| {
                let p = chrome::palette(ui);
                ui.label(
                    egui::RichText::new(format!("Settings file  {}", AppSettings::path_display()))
                        .color(p.muted)
                        .size(13.0),
                );
                if let Some(s) = &self.session {
                    ui.add_space(6.0);
                    ui.label(chrome::semi(
                        format!("{}  ·  {}", s.user.full_name, s.user.role.replace('_', " ")),
                        14.0,
                        p.ink,
                    ));
                    ui.label(egui::RichText::new(&s.user.username).color(p.muted).size(13.0));
                }
            });
            ui.add_space(12.0);
            chrome::form_card(ui, "Shortcuts", |ui| {
                let p = chrome::palette(ui);
                let m = if cfg!(target_os = "macos") { "⌘" } else { "Ctrl+" };
                for line in [
                    format!("{m}1 to {m}5    Floor, inspections, alerts, actions, settings"),
                    format!("{m}R    Refresh the page you are on"),
                    format!("{m}N    New record on a list"),
                    format!("{m},    Settings"),
                    "Esc    Close a form or a detail".to_string(),
                ] {
                    ui.label(egui::RichText::new(line).color(p.ink).size(13.5));
                }
            });
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
        chrome::form_card(ui, "New inspection", |ui| {
            chrome::field_label(ui, "Build (lot)");
            let products = &self.products;
            let lot_label = |l: &Lot| {
                let product = products
                    .iter()
                    .find(|p| p.id == l.product_id)
                    .map(|p| p.name.as_str())
                    .unwrap_or("");
                format!("{}  {}  ({})", l.lot_number, product, l.status.replace('_', " "))
            };
            if self.lots.iter().all(|l| matches!(l.status.as_str(), "closed" | "on_hold")) {
                chrome::muted(ui, "No lots ready to inspect. Add one on the Lots page, or release a hold.");
            } else {
                egui::ComboBox::from_id_salt("lot_pick")
                    .width(ui.available_width())
                    .height(360.0)
                    .selected_text(
                        self.lots
                            .iter()
                            .find(|l| l.id.to_string() == self.insp_form.lot_id)
                            .map(lot_label)
                            .unwrap_or_else(|| "Pick the build to check".to_string()),
                    )
                    .show_ui(ui, |ui| {
                        for lot in self.lots.iter().filter(|l| !matches!(l.status.as_str(), "closed" | "on_hold")) {
                            if ui
                                .selectable_label(self.insp_form.lot_id == lot.id.to_string(), lot_label(lot))
                                .clicked()
                            {
                                self.insp_form.lot_id = lot.id.to_string();
                            }
                        }
                    });
                ui.add_space(4.0);
                chrome::muted(ui, "The product's active plan sets the checks.");
            }
            ui.add_space(8.0);
            chrome::field_label(ui, "Notes");
            ui.add(
                egui::TextEdit::multiline(&mut self.insp_form.notes)
                    .desired_width(f32::INFINITY)
                    .desired_rows(3),
            );
            ui.add_space(12.0);
            ui.horizontal(|ui| {
                if chrome::button(ui, "Create", Btn::Primary).clicked() {
                    self.submit_inspection();
                }
                if chrome::button(ui, "Cancel", Btn::Quiet).clicked() {
                    self.form = FormMode::None;
                }
            });
            if let Some(err) = &self.insp_form.error {
                ui.add_space(8.0);
                error_banner(ui, err);
            }
        });
        ui.add_space(12.0);
    }

    fn ui_ncr_form(&mut self, ui: &mut egui::Ui) {
        let title = if matches!(self.form, FormMode::EditNcr(_)) {
            "Edit alert"
        } else {
            "New alert"
        };
        chrome::form_card(ui, title, |ui| {
            chrome::field_label(ui, "Product");
            if !self.products.is_empty() {
                egui::ComboBox::from_id_salt("product_pick")
                    .width(ui.available_width())
                    .selected_text(if self.ncr_form.product_id.is_empty() {
                        "Select a product".to_string()
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
                ui.add_space(8.0);
            }
            chrome::field_label(ui, "Title");
            chrome::text_line(ui, &mut self.ncr_form.title);
            ui.add_space(8.0);
            chrome::field_label(ui, "Description");
            ui.add(
                egui::TextEdit::multiline(&mut self.ncr_form.description)
                    .desired_width(f32::INFINITY)
                    .desired_rows(3),
            );
            ui.add_space(8.0);
            chrome::field_label(ui, "Severity");
            ui.horizontal(|ui| {
                for s in ["minor", "major", "critical"] {
                    ui.radio_value(&mut self.ncr_form.severity, s.to_string(), s);
                }
            });
            ui.add_space(8.0);
            chrome::field_label(ui, "Quantity affected");
            chrome::text_line(ui, &mut self.ncr_form.quantity);
            ui.add_space(12.0);
            ui.horizontal(|ui| {
                if chrome::button(ui, "Save", Btn::Primary).clicked() {
                    self.submit_ncr();
                }
                if chrome::button(ui, "Cancel", Btn::Quiet).clicked() {
                    self.form = FormMode::None;
                }
            });
            if let Some(err) = &self.ncr_form.error {
                ui.add_space(8.0);
                error_banner(ui, err);
            }
        });
        ui.add_space(12.0);
    }

    fn ui_capa_form(&mut self, ui: &mut egui::Ui) {
        let title = if matches!(self.form, FormMode::EditCapa(_)) {
            "Edit action"
        } else {
            "New action"
        };
        chrome::form_card(ui, title, |ui| {
            if let Some((_, number)) = &self.capa_form.ncr {
                chrome::muted(ui, format!("For alert {number}"));
                ui.add_space(6.0);
            }
            chrome::field_label(ui, "Title");
            chrome::text_line(ui, &mut self.capa_form.title);
            ui.add_space(8.0);
            chrome::field_label(ui, "Root cause");
            ui.add(
                egui::TextEdit::multiline(&mut self.capa_form.root_cause)
                    .desired_width(f32::INFINITY)
                    .desired_rows(3),
            );
            ui.add_space(8.0);
            chrome::field_label(ui, "Action plan");
            ui.add(
                egui::TextEdit::multiline(&mut self.capa_form.action_plan)
                    .desired_width(f32::INFINITY)
                    .desired_rows(3),
            );
            ui.add_space(8.0);
            chrome::field_label(ui, "Due date");
            chrome::text_line(ui, &mut self.capa_form.due_date);
            ui.add_space(12.0);
            ui.horizontal(|ui| {
                if chrome::button(ui, "Save", Btn::Primary).clicked() {
                    self.submit_capa();
                }
                if chrome::button(ui, "Cancel", Btn::Quiet).clicked() {
                    self.form = FormMode::None;
                    self.capa_form.ncr = None;
                }
            });
            if let Some(err) = &self.capa_form.error {
                ui.add_space(8.0);
                error_banner(ui, err);
            }
        });
        ui.add_space(12.0);
    }

}

fn column_sort(title: &str) -> Option<SortKey> {
    match title {
        "Number" => Some(SortKey::Number),
        "Status" => Some(SortKey::Status),
        "Result" => Some(SortKey::Result),
        "Started" => Some(SortKey::Opened),
        "Title" => Some(SortKey::Title),
        "Severity" => Some(SortKey::Severity),
        "Due" => Some(SortKey::Due),
        _ => None,
    }
}

fn record_table(
    ui: &mut egui::Ui,
    headers: &[&str],
    len: usize,
    mut draw: impl FnMut(&mut egui::Ui, usize, usize),
) -> Option<String> {
    use egui_extras::{Column, TableBuilder};
    let p = chrome::palette(ui);
    let mut clicked = None;
    TableBuilder::new(ui)
        .striped(true)
        .resizable(true)
        .cell_layout(egui::Layout::left_to_right(egui::Align::Center))
        .columns(Column::remainder().at_least(88.0), headers.len())
        .header(32.0, |mut header| {
            for title in headers {
                header.col(|ui| {
                    let response = ui.add(
                        egui::Label::new(chrome::semi(*title, 12.0, p.muted))
                            .sense(egui::Sense::click()),
                    );
                    if response.clicked() {
                        clicked = Some((*title).to_string());
                    }
                });
            }
        })
        .body(|body| {
            body.rows(42.0, len, |mut row| {
                let idx = row.index();
                for col in 0..headers.len() {
                    row.col(|ui| draw(ui, idx, col));
                }
            });
        });
    clicked
}
