export default {
  extends: ["stylelint-config-standard"],
  rules: {
    "at-rule-empty-line-before": null,
    "color-named": "never",
    "color-no-hex": true,
    "custom-property-empty-line-before": null,
    "declaration-block-single-line-max-declarations": null,
    "function-disallowed-list": ["rgb", "rgba", "hsl", "hsla"],
    "hue-degree-notation": "angle",
    "media-feature-range-notation": "context",
    "no-descending-specificity": null,
  },
};
