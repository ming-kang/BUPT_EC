import PropTypes from "prop-types";
import { Button } from "antd";
import { Fragment, Suspense, lazy, useState } from "react";
import { SettingIcon } from "./icons";
import { useSelection } from "../selectionContext";
import "./CampusButtonGroup.css";

const CampusSettingsModal = lazy(() => import("./CampusSettingsModal"));

function CampusButtonGroup({ campuses, todayData, activeCampusId }) {
  const { dispatch } = useSelection();
  const [openSettingModal, setOpenSettingModal] = useState(false);
  const list = Array.isArray(campuses) ? campuses : [];
  const settingsSplitIndex = Math.floor(list.length / 2);

  return (
    <div className="campus-button-group">
      <div className="campus-buttons">
        {list.length === 0 ? (
          <Button
            className="settings-trigger"
            icon={<SettingIcon />}
            onClick={() => setOpenSettingModal(true)}
            aria-label="设置"
          />
        ) : (
          list.map((campus, index) => (
            <Fragment key={campus.id}>
              {index === settingsSplitIndex ? (
                <Button
                  className="settings-trigger"
                  icon={<SettingIcon />}
                  onClick={() => setOpenSettingModal(true)}
                  aria-label="设置"
                />
              ) : null}
              <Button
                type={activeCampusId === campus.id ? "primary" : "default"}
                // R12: single-select group, but kept on aria-pressed like the
                // other pickers (design D10) instead of a radiogroup rewrite.
                aria-pressed={activeCampusId === campus.id}
                onClick={() => dispatch({ type: "SET_CAMPUS", id: campus.id })}
              >
                {campus.name}
              </Button>
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

CampusButtonGroup.propTypes = {
  campuses: PropTypes.arrayOf(
    PropTypes.shape({
      id: PropTypes.string.isRequired,
      name: PropTypes.string.isRequired,
    })
  ).isRequired,
  todayData: PropTypes.object.isRequired,
  // Render-time derived campus id from App (R10): first frame already
  // highlights the effective campus before the store reconciles.
  activeCampusId: PropTypes.string.isRequired,
};

export default CampusButtonGroup;
