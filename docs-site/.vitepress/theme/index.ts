import DefaultTheme from "vitepress/theme";
import "./style.css";

// Theme exports. T6 will register the BetaBanner component in this file
// when it lands; until then we just extend the default theme with brand
// tokens defined in style.css.
export default {
  extends: DefaultTheme,
};
