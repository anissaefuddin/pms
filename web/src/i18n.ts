import i18n from "i18next";
import { initReactI18next } from "react-i18next";
import apps from "./i18n/en/apps.json";
import audit from "./i18n/en/audit.json";
import auth from "./i18n/en/auth.json";
import common from "./i18n/en/common.json";
import automation from "./i18n/en/automation.json";
import dashboards from "./i18n/en/dashboards.json";
import blueprints from "./i18n/en/blueprints.json";
import excel from "./i18n/en/excel.json";
import forms from "./i18n/en/forms.json";
import gantt from "./i18n/en/gantt.json";
import items from "./i18n/en/items.json";
import nav from "./i18n/en/nav.json";
import oauth from "./i18n/en/oauth.json";
import reports from "./i18n/en/reports.json";
import ownership from "./i18n/en/ownership.json";
import policy from "./i18n/en/policy.json";
import portfolio from "./i18n/en/portfolio.json";
import workload from "./i18n/en/workload.json";
import resources from "./i18n/en/resources.json";
import provisioning from "./i18n/en/provisioning.json";
import roles from "./i18n/en/roles.json";
import search from "./i18n/en/search.json";
import settings from "./i18n/en/settings.json";
import share from "./i18n/en/share.json";
import spaces from "./i18n/en/spaces.json";
import sso from "./i18n/en/sso.json";
import starred from "./i18n/en/starred.json";
import time from "./i18n/en/time.json";
import views from "./i18n/en/views.json";
import webhooks from "./i18n/en/webhooks.json";
import workspace from "./i18n/en/workspace.json";
import idApps from "./i18n/id/apps.json";
import idAudit from "./i18n/id/audit.json";
import idAuth from "./i18n/id/auth.json";
import idCommon from "./i18n/id/common.json";
import idAutomation from "./i18n/id/automation.json";
import idDashboards from "./i18n/id/dashboards.json";
import idBlueprints from "./i18n/id/blueprints.json";
import idExcel from "./i18n/id/excel.json";
import idForms from "./i18n/id/forms.json";
import idGantt from "./i18n/id/gantt.json";
import idItems from "./i18n/id/items.json";
import idNav from "./i18n/id/nav.json";
import idOauth from "./i18n/id/oauth.json";
import idReports from "./i18n/id/reports.json";
import idOwnership from "./i18n/id/ownership.json";
import idPolicy from "./i18n/id/policy.json";
import idPortfolio from "./i18n/id/portfolio.json";
import idWorkload from "./i18n/id/workload.json";
import idResources from "./i18n/id/resources.json";
import idProvisioning from "./i18n/id/provisioning.json";
import idRoles from "./i18n/id/roles.json";
import idSearch from "./i18n/id/search.json";
import idSettings from "./i18n/id/settings.json";
import idShare from "./i18n/id/share.json";
import idSpaces from "./i18n/id/spaces.json";
import idSso from "./i18n/id/sso.json";
import idStarred from "./i18n/id/starred.json";
import idTime from "./i18n/id/time.json";
import idViews from "./i18n/id/views.json";
import idWebhooks from "./i18n/id/webhooks.json";
import idWorkspace from "./i18n/id/workspace.json";

export const LANGUAGES = ["en", "id"] as const;
export type Language = (typeof LANGUAGES)[number];
const LANGUAGE_KEY = "pms-language";

export function isLanguage(value: unknown): value is Language {
  return LANGUAGES.includes(value as Language);
}

/** Before login: the last choice saved in this browser, else the browser language, else English. */
function initialLanguage(): Language {
  try {
    const stored = localStorage.getItem(LANGUAGE_KEY);
    if (isLanguage(stored)) return stored;
  } catch {
    // Storage blocked (private window or disabled site data): fall through to the browser language.
  }
  return navigator.language?.toLowerCase().startsWith("id") ? "id" : "en";
}

/** Switches the UI language live and remembers it in this browser. The server copy is saved by the caller. */
export function setLanguage(language: Language) {
  try {
    localStorage.setItem(LANGUAGE_KEY, language);
  } catch {
    // Not persisted here; the language still changes for this page view.
  }
  document.documentElement.lang = language;
  return i18n.changeLanguage(language);
}

const initial = initialLanguage();
document.documentElement.lang = initial;

i18n.use(initReactI18next).init({
  lng: initial,
  fallbackLng: "en",
  interpolation: { escapeValue: false },
  resources: {
    en: {
      translation: { apps, audit, auth, automation, blueprints, common, dashboards, excel, forms, gantt, items, nav, oauth, ownership, policy, portfolio, provisioning, reports, resources, roles, search, settings, share, spaces, sso, starred, time, views, webhooks, workload, workspace },
    },
    id: {
      translation: { apps: idApps, audit: idAudit, auth: idAuth, automation: idAutomation, blueprints: idBlueprints, common: idCommon, dashboards: idDashboards, excel: idExcel, forms: idForms, gantt: idGantt, items: idItems, nav: idNav, oauth: idOauth, ownership: idOwnership, policy: idPolicy, portfolio: idPortfolio, provisioning: idProvisioning, reports: idReports, resources: idResources, roles: idRoles, search: idSearch, settings: idSettings, share: idShare, spaces: idSpaces, sso: idSso, starred: idStarred, time: idTime, views: idViews, webhooks: idWebhooks, workload: idWorkload, workspace: idWorkspace },
    },
  },
});

export default i18n;
