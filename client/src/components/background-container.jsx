import React from 'react';
import { Motion, spring } from 'react-motion';

export class BackgroundContainer extends React.Component {

  constructor() {
    super();

    this.state = {
      shiftX: 50,
      shiftY: 60
    };
  }

  render() {
    const backgroundPosition = `${this.state.shiftX}% ${this.state.shiftY}%`;
    const motionConfig = [200, 20];
    const styles = {
      background: {
        backgroundImage: `url("${this.props.imageUrl}")`,
        backgroundPosition: backgroundPosition,
        backgroundRepeat: 'no-repeat',
        position: 'fixed',
        top: 0,
        bottom: 0,
        left: 0,
        right: 0,
        zIndex: -10
      },
      container: {
        height: '100%'
      },
      motion: {
        shiftX: spring(this.state.shiftX, motionConfig),
        shiftY: spring(this.state.shiftY, motionConfig)
      }
    };
    const imageUrl = this.props.imageUrl;

    return (
      <div style={styles.container} onMouseMove={this.handleMouseMove.bind(this)}>
        <Motion style={styles.motion}>
          {({shiftX, shiftY}) =>
            <div style={{
              backgroundImage: `url("${imageUrl}")`,
              backgroundPosition: `${shiftX}% ${shiftY}%`,
              backgroundRepeat: 'no-repeat',
              position: 'fixed',
              top: 0,
              bottom: 0,
              left: 0,
              right: 0,
              zIndex: -10
            }}></div>
          }
        </Motion>
        {this.props.children}
      </div>
    );
  }

  handleMouseMove(e) {
    const centerX = window.innerWidth / 2;
    const centerY = window.innerHeight / 2;
    const maxShiftPercent = 5;
    const shiftX = 50 + (centerX - e.clientX) / centerX * maxShiftPercent;
    const shiftY = 60 + (centerY - e.clientY) / centerY * maxShiftPercent;

    this.setState({shiftX: shiftX, shiftY: shiftY});
  }
}
