import DefaultTheme from "vitepress/theme";
import { useOpenapi } from "vitepress-openapi/client";
import "vitepress-openapi/dist/style.css";
import "./style.css";
import BetaBanner from "../components/BetaBanner.vue";

export default {
  extends: DefaultTheme,
  enhanceApp({ app }) {
    useOpenapi();
    app.component("BetaBanner", BetaBanner);
  },
};
