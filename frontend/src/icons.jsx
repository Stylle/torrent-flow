import React from "react";
import { FontAwesomeIcon } from "@fortawesome/react-fontawesome";
import { config } from "@fortawesome/fontawesome-svg-core";
import "@fortawesome/fontawesome-svg-core/styles.css";
import {
  faWaveSquare,
  faGauge,
  faChartColumn,
  faSliders,
  faDownload,
  faGear,
  faSun,
  faMoon,
  faDesktop,
  faBars,
  faXmark,
  faRightFromBracket,
  faPlus,
  faArrowUp,
  faArrowDown,
  faTowerBroadcast,
  faShieldHalved,
  faCircleInfo,
  faRotateRight,
  faTrashCan,
  faLink,
  faFloppyDisk,
  faGlobe,
  faArrowRight,
} from "@fortawesome/free-solid-svg-icons";
config.autoAddCss = false;
const wrap = (icon) =>
  function Icon({ size = 16, style, ...props }) {
    return (
      <FontAwesomeIcon
        icon={icon}
        style={{ fontSize: size, ...style }}
        {...props}
      />
    );
  };
export const WaveSine = wrap(faWaveSquare),
  House = wrap(faGauge),
  ChartBar = wrap(faChartColumn),
  Stack = wrap(faSliders),
  DownloadSimple = wrap(faDownload),
  GearSix = wrap(faGear),
  Sun = wrap(faSun),
  Moon = wrap(faMoon),
  Desktop = wrap(faDesktop),
  List = wrap(faBars),
  X = wrap(faXmark),
  SignOut = wrap(faRightFromBracket),
  Plus = wrap(faPlus),
  ArrowUp = wrap(faArrowUp),
  ArrowDown = wrap(faArrowDown),
  Broadcast = wrap(faTowerBroadcast),
  ShieldCheck = wrap(faShieldHalved),
  Info = wrap(faCircleInfo),
  ArrowClockwise = wrap(faRotateRight),
  Trash = wrap(faTrashCan),
  Link = wrap(faLink),
  FloppyDisk = wrap(faFloppyDisk),
  Globe = wrap(faGlobe),
  ArrowRight = wrap(faArrowRight);
