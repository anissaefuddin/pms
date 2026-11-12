import parser from "@typescript-eslint/parser";
import i18next from "eslint-plugin-i18next";

export default [
  {
    files: ["src/**/*.tsx"],
    languageOptions: { parser },
    plugins: { i18next },
    rules: {
      "i18next/no-literal-string": ["error", {
        mode: "jsx-only",
        "jsx-attributes": {
          include: ["^(aria-label|aria-description|alt|label|placeholder|title)$"],
        },
        "should-validate-template": true,
      }],
    },
  },
];
