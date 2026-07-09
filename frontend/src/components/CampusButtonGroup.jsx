import PropTypes from "prop-types";
import { Button } from "antd";
import { Fragment, Suspense, lazy, useState } from "react";
import { SettingOutlined } from "@ant-design/icons";
import { useSelection } from "../selectionContext";
import "./CampusButtonGroup.css";

const CampusSettingsModal = lazy(() => import("./CampusSettingsModal"));

function CampusButtonGroup({ campuses, todayData }) {
  const { state, dispatch } = useSelection();
  const [openSettingModal, setOpenSettingModal] = useState(false);
  const list = Array.isArray(campuses) ? campuses : [];
  const settingsSplitIndex = Math.floor(list.length / 2);

  return (
    <div className="campus-button-group">
      <div className="campus-buttons">
        {list.length === 0 ? (
          <Button
            className="settings-trigger"
            icon={<SettingOutlined />}
            onClick={() => setOpenSettingModal(true)}
            aria-label="设置"
          />
        ) : (
          list.map((campus, index) => (
            <Fragment key={campus.id}>
              {index === settingsSplitIndex ? (
                <Button
                  className="settings-trigger"
                  icon={<SettingOutlined />}
                  onClick={() => setOpenSettingModal(true)}
                  aria-label="设置"
                />
              ) : null}
              <Button
                type={state.selectedCampus === campus.id ? "primary" : "default"}
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
};

export default CampusButtonGroup;
