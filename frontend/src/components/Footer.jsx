import { Typography, Button } from "antd";
import { GithubOutlined } from "@ant-design/icons";

function Footer() {
  const { Text } = Typography;
  return (
    <div className="app-footer">
      <Text type="secondary">© 2026~ ming-kang</Text>
      <Button
        onClick={() =>
          window.open(
            "https://github.com/ming-kang/BUPT_EC",
            "_blank",
            "noopener,noreferrer"
          )
        }
        type="text"
        size="small"
        icon={<GithubOutlined />}
        aria-label="GitHub"
      />
    </div>
  );
}

export default Footer;
