import i18n from "i18next";
import { initReactI18next } from "react-i18next";
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
import search from "./i18n/en/search.json";
import settings from "./i18n/en/settings.json";
import share from "./i18n/en/share.json";
import spaces from "./i18n/en/spaces.json";
import starred from "./i18n/en/starred.json";
import time from "./i18n/en/time.json";
import views from "./i18n/en/views.json";
import workspace from "./i18n/en/workspace.json";

i18n.use(initReactI18next).init({
  lng: "en",
  fallbackLng: "en",
  interpolation: { escapeValue: false },
  resources: {
    en: {
      translation: { auth, automation, blueprints, common, dashboards, excel, forms, gantt, items, nav, oauth, reports, search, settings, share, spaces, starred, time, views, workspace },
    },
  },
});

export default i18n;
