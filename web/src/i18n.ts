import i18n from "i18next";
import { initReactI18next } from "react-i18next";
import auth from "./i18n/en/auth.json";
import common from "./i18n/en/common.json";
import gantt from "./i18n/en/gantt.json";
import nav from "./i18n/en/nav.json";
import settings from "./i18n/en/settings.json";
import workspace from "./i18n/en/workspace.json";

i18n.use(initReactI18next).init({
  lng: "en",
  fallbackLng: "en",
  interpolation: { escapeValue: false },
  resources: {
    en: {
      translation: { auth, common, gantt, nav, settings, workspace },
    },
  },
});

export default i18n;
