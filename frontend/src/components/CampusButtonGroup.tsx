import { Button } from "antd";
import type { Envelope } from "../api/decode";
import type { CampusInfo } from "../api/types";
import { Fragment, Suspense, lazy, useState } from "react";
import { SettingIcon } from "./icons";
import { useSelection } from "../selectionContext";
import { ToggleButton } from "./ToggleButtonGroup";
import "./CampusButtonGroup.css";

const CampusSettingsModal = lazy(() => import("./CampusSettingsModal"));

interface CampusButtonGroupProps {
  campuses: CampusInfo[];
  todayData: Envelope;
  /** Render-time derived campus id from App (R10): first frame already
   * highlights the effective campus before the store reconciles. */
  activeCampusId: string;
}

function CampusButtonGroup({ campuses, todayData, activeCampusId }: CampusButtonGroupProps) {
  const { setCampus } = useSelection();
  const [openSettingModal, setOpenSettingModal] = useState(false);
  const settingsSplitIndex = Math.floor(campuses.length / 2);

  return (
    <div className="campus-button-group">
      <div className="toggle-button-group campus-buttons">
        {campuses.length === 0 ? (
          <Button
            className="settings-trigger"
            icon={<SettingIcon />}
            onClick={() => setOpenSettingModal(true)}
            aria-label="设置"
          />
        ) : (
          campuses.map((campus, index) => (
            <Fragment key={campus.id}>
              {index === settingsSplitIndex ? (
                <Button
                  className="settings-trigger"
                  icon={<SettingIcon />}
                  onClick={() => setOpenSettingModal(true)}
                  aria-label="设置"
                />
              ) : null}
              <ToggleButton
                // R12: single-select group, but kept on aria-pressed like the
                // other pickers (design D10) instead of a radiogroup rewrite.
                pressed={activeCampusId === campus.id}
                onClick={() => setCampus(campus.id)}
              >
                {campus.name}
              </ToggleButton>
            </Fragment>
          ))
        )}
      </div>
      {openSettingModal ? (
        <Suspense fallback={null}>
          <CampusSettingsModal
            open={openSettingModal}
            todayData={todayData}
            onClose={() => setOpenSettingModal(false)}
          />
        </Suspense>
      ) : null}
    </div>
  );
}

export default CampusButtonGroup;
