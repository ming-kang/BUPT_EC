import { Typography, Button } from "antd";
import { GithubOutlined } from "@ant-design/icons";

function Footer() {
  const { Text } = Typography;
  return (
    <Text className="footer">
      © 2022-2026 ming-kang
      <Button
        onClick={() =>
          window.open("https://github.com/ming-kang/BUPT_EC")
        }
        type="text"
        icon={<GithubOutlined />}
      ></Button>
    </Text>
  );
}

export default Footer;
